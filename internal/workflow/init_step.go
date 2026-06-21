package workflow

import (
	"context"
	"fmt"

	"github.com/zolagz/ytb2bili/internal/config"
	"github.com/zolagz/ytb2bili/internal/service"
	"github.com/zolagz/ytb2bili/pkg/llm"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// ============================================================================
// 步骤 1: 初始化
// ============================================================================

type InitStep struct {
	BaseStep
	logger       *zap.Logger
	cfg          config.WorkflowConfig
	userSettings *service.UserSettingsClient
}

type InitStepParams struct {
	fx.In
	Logger       *zap.Logger
	Cfg          config.WorkflowConfig
	UserSettings *service.UserSettingsClient `optional:"true"`
}

func NewInitStep(params InitStepParams) *InitStep {
	return &InitStep{
		BaseStep:     NewBaseStepWithOrder(StepNameInitialize, true, 1),
		logger:       params.Logger,
		cfg:          params.Cfg,
		userSettings: params.UserSettings,
	}
}

func (s *InitStep) Execute(ctx context.Context, input any) (any, error) {
	// 直接传入 VideoContext（本地视频场景），补全默认配置后透传
	if vctx, ok := input.(*VideoContext); ok {
		if vctx.TranslationConfig == nil {
			sourceLang := s.cfg.LLMTranslationSourceLang
			if sourceLang == "" {
				sourceLang = "en"
			}
			targetLang := s.cfg.LLMTranslationTargetLang
			if targetLang == "" {
				targetLang = "zh-Hans"
			}
			vctx.TranslationConfig = &TranslationConfig{
				SourceLanguage: sourceLang,
				TargetLanguage: targetLang,
				ModelName:      llm.DefaultTranslationModel,
			}
		}
		if vctx.TranslationConfig.ModelName == "" {
			vctx.TranslationConfig.ModelName = llm.DefaultTranslationModel
		}
		if vctx.SpeechSynthesisConfig == nil {
			vctx.SpeechSynthesisConfig = &SpeechSynthesisConfig{
				Language:  "zh-CN",
				VoiceName: "zh-CN-XiaoxiaoNeural",
				Format:    "mp3",
			}
		}
		// 若 VideoContext 没有携带 UserID，尝试从 context 中补全（供积分扣减使用）
		if vctx.UserID == "" {
			if uid := GetUserID(ctx); uid != "" {
				vctx.UserID = uid
			}
		}
		if !preferencesApplied(ctx) {
			applyLatestUserSettingsToVideoContext(ctx, s.userSettings, s.logger, vctx)
		}
		return vctx, nil
	}

	videoURL, ok := input.(string)
	if !ok {
		return nil, fmt.Errorf("expected string input (video URL), got %T", input)
	}

	// 从 context 中读取用户 ID（由 ProcessWithTracking 注入）
	userID := GetUserID(ctx)

	vctx := &VideoContext{
		VideoURL: videoURL,
		UserID:   userID,
		// 设置默认翻译配置
		TranslationConfig: &TranslationConfig{
			SourceLanguage: func() string {
				if s.cfg.LLMTranslationSourceLang != "" {
					return s.cfg.LLMTranslationSourceLang
				}
				return "en"
			}(),
			TargetLanguage: func() string {
				if s.cfg.LLMTranslationTargetLang != "" {
					return s.cfg.LLMTranslationTargetLang
				}
				return "zh-Hans"
			}(),
			ModelName: llm.DefaultTranslationModel,
		},
		// 设置默认语音合成配置
		SpeechSynthesisConfig: &SpeechSynthesisConfig{
			Language:  "zh-CN",
			VoiceName: "zh-CN-XiaoxiaoNeural", // 晓晓（女声）
			Format:    "mp3",
		},
	}
	if !preferencesApplied(ctx) {
		applyLatestUserSettingsToVideoContext(ctx, s.userSettings, s.logger, vctx)
	}
	return vctx, nil
}

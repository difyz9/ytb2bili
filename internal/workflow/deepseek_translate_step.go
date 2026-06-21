// Package workflow — DeepseekTranslateStep is a thin wrapper around LLMTranslateStep.
// It uses the unified BatchTranslator (backed by any provider) just like LLMTranslateStep.
//
// Both step names map to the frontend label "LLM 字幕翻译" — the underlying
// provider is controlled purely by config ([translation] or [llm]).
//
// This file exists only for backward compatibility with workflows that still
// reference DeepseekTranslateStep. New code should use LLMTranslateStep directly.
package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zolagz/ytb2bili/internal/config"
	"github.com/zolagz/ytb2bili/internal/service"
	"github.com/zolagz/ytb2bili/pkg/llm"
	storemodel "github.com/zolagz/ytb2bili/pkg/store/model"
	"github.com/zolagz/ytb2bili/pkg/tools"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// DeepseekTranslateStep is a thin wrapper over LLMTranslateStep for backward compat.
// It creates a BatchTranslator from the configured translation provider.
type DeepseekTranslateStep struct {
	BaseStep
	translator   *tools.BatchTranslator
	logger       *zap.Logger
	userSettings *service.UserSettingsClient
	appConfig    *config.AppConfig
}

type DeepseekTranslateStepParams struct {
	fx.In
	Translator   *tools.BatchTranslator       `optional:"true"`
	Logger       *zap.Logger
	UserSettings *service.UserSettingsClient `optional:"true"`
	AppConfig    *config.AppConfig           `optional:"true"`
}

func NewDeepseekTranslateStep(params DeepseekTranslateStepParams) *DeepseekTranslateStep {
	translator := params.Translator

	// 如果没有注入 translator，尝试从配置创建（使用统一的 translation provider）
	if translator == nil && params.AppConfig != nil {
		chatLLM, err := provideLLMClientForTranslation(params.AppConfig, params.Logger)
		if err == nil && chatLLM != nil {
			wf := params.AppConfig.Workflow
			translator = tools.NewBatchTranslator(chatLLM, tools.BatchTranslatorConfig{
				SourceLang:  wf.LLMTranslationSourceLang,
				TargetLang:  wf.LLMTranslationTargetLang,
				BatchSize:   wf.LLMTranslationBatchSize,
				MaxWorkers:  wf.LLMTranslationMaxWorkers,
				ContextSize: wf.LLMTranslationContextSize,
			}, params.Logger)
			params.Logger.Info("Created BatchTranslator from app config (for backward compat deepseek step)")
		}
	}

	return &DeepseekTranslateStep{
		BaseStep:     NewBaseStepWithOrder(StepNameLLMTranslate, false, 6),
		translator:   translator,
		logger:       params.Logger,
		userSettings: params.UserSettings,
		appConfig:    params.AppConfig,
	}
}

// provideLLMClientForTranslation 从 translation provider 或回退到 chat provider 创建 LLM 客户端
func provideLLMClientForTranslation(cfg *config.AppConfig, logger *zap.Logger) (*llm.EinoChatClient, error) {
	p := cfg.ResolveTranslationProvider()
	if p == nil || !p.ToLLMConfig().IsValid() {
		// Fallback to chat provider
		p = cfg.ResolveChatProvider()
	}
	if p == nil || !p.ToLLMConfig().IsValid() {
		logger.Warn("No LLM configured for translation (neither [translation] nor [chat] nor [llm])")
		return nil, nil
	}
	return llm.NewClientFromConfig(p.ToLLMConfig(), logger)
}

func (s *DeepseekTranslateStep) Execute(ctx context.Context, input any) (any, error) {
	vctx, err := mustVideoContext(input)
	if err != nil {
		return nil, err
	}

	if s.translator == nil {
		s.logger.Warn("Subtitle translator unavailable, skipping translation step")
		return vctx, nil
	}

	segments := collectTranscriptTextSegments(vctx.Transcript)
	if len(segments) == 0 {
		s.logger.Warn("No transcript available, skipping subtitle translation")
		return vctx, nil
	}

	ctx = s.refreshTranslationConfig(ctx, vctx)

	s.logger.Info("Starting subtitle translation",
		zap.Int("total_segments", len(segments)),
		zap.String("source_lang", vctx.TranslationConfig.SourceLanguage),
		zap.String("target_lang", vctx.TranslationConfig.TargetLanguage))

	texts := transcriptTexts(segments)
	if len(texts) == 0 {
		s.logger.Warn("No text to translate")
		return vctx, nil
	}

	runConfig := tools.TranslationRunConfig{
		SourceLang: vctx.TranslationConfig.SourceLanguage,
		TargetLang: vctx.TranslationConfig.TargetLanguage,
		ModelName:  vctx.TranslationConfig.ModelName,
		UserID:     strings.TrimSpace(vctx.UserID),
	}

	result, err := s.translator.TranslateTextsWithConfig(ctx, texts, runConfig)
	if err != nil {
		s.logger.Error("Subtitle translation failed", zap.Error(err))
		return vctx, &StepSkippedError{
			Step:   s.Name(),
			Cause:  err,
			Output: vctx,
		}
	}

	vctx.TranslationSkipped = result.SkippedTranslation
	vctx.SubtitleAudios = buildSubtitleAudiosFromTranslations(segments, result.TranslatedTexts)

	s.logger.Info("Subtitle translation completed",
		zap.Int("total_segments", len(segments)),
		zap.Int("translated_count", len(vctx.SubtitleAudios)),
		zap.Bool("translation_skipped", result.SkippedTranslation),
		zap.Duration("duration", result.Duration))

	if err := s.saveTranslatedSubtitles(vctx); err != nil {
		s.logger.Error("Failed to save translated subtitles", zap.Error(err))
		return vctx, &StepSkippedError{
			Step:   s.Name(),
			Cause:  err,
			Output: vctx,
		}
	}

	return vctx, nil
}

func (s *DeepseekTranslateStep) ShouldSkip(ctx context.Context, input any) bool {
	if vctx, ok := input.(*VideoContext); ok {
		settings := NormalizeTaskChainSettings(vctx.TaskChainSettings)
		return !settings.TranslateSubtitles
	}
	return false
}

func (s *DeepseekTranslateStep) refreshTranslationConfig(ctx context.Context, vctx *VideoContext) context.Context {
	if vctx.TranslationConfig == nil {
		vctx.TranslationConfig = &TranslationConfig{}
	}
	if strings.TrimSpace(vctx.TranslationConfig.SourceLanguage) == "" {
		vctx.TranslationConfig.SourceLanguage = "en"
	}
	if strings.TrimSpace(vctx.TranslationConfig.TargetLanguage) == "" {
		vctx.TranslationConfig.TargetLanguage = "zh-Hans"
	}
	if strings.TrimSpace(vctx.TranslationConfig.ModelName) == "" {
		vctx.TranslationConfig.ModelName = llm.DefaultTranslationModel
	}

	userID := strings.TrimSpace(vctx.UserID)
	if userID == "" {
		userID = strings.TrimSpace(GetUserID(ctx))
		if userID != "" {
			vctx.UserID = userID
		}
	}

	if s.userSettings != nil && s.userSettings.IsEnabled() && userID != "" {
		settings, err := s.userSettings.GetSettings(ctx, userID)
		if err != nil {
			s.logger.Warn("Failed to refresh translation settings from database",
				zap.String("user_id", userID), zap.Error(err))
		} else {
			if sourceLang := strings.TrimSpace(settings[storemodel.UserSettingKeyTranslationSourceLang]); sourceLang != "" {
				vctx.TranslationConfig.SourceLanguage = sourceLang
			}
			if targetLang := strings.TrimSpace(settings[storemodel.UserSettingKeyTranslationTargetLang]); targetLang != "" {
				vctx.TranslationConfig.TargetLanguage = targetLang
			}
			if modelName := resolvePreferredWorkflowModel(settings, storemodel.UserSettingKeyTranslationModel); modelName != "" {
				vctx.TranslationConfig.ModelName = modelName
			}
		}
	}
	return ctx
}

func (s *DeepseekTranslateStep) saveTranslatedSubtitles(vctx *VideoContext) error {
	if len(vctx.SubtitleAudios) == 0 {
		return nil
	}

	videoDir := filepath.Dir(vctx.VideoPath)

	englishPath := filepath.Join(videoDir, "subtitles_en.srt")
	if err := s.writeSRTFile(englishPath, vctx.SubtitleAudios, false); err != nil {
		return fmt.Errorf("save English subtitles: %w", err)
	}
	s.logger.Info("Saved English subtitles", zap.String("path", englishPath))

	translatedPath := filepath.Join(videoDir, "subtitles_translated.srt")
	if err := s.writeSRTFile(translatedPath, vctx.SubtitleAudios, true); err != nil {
		return fmt.Errorf("save translated subtitles: %w", err)
	}
	s.logger.Info("Saved translated subtitles", zap.String("path", translatedPath))

	return nil
}

func (s *DeepseekTranslateStep) writeSRTFile(path string, subtitles []SubtitleAudio, useTranslated bool) error {
	var content strings.Builder
	for i, sub := range subtitles {
		content.WriteString(fmt.Sprintf("%d\n", i+1))
		startTime := s.formatSRTTime(sub.StartTime)
		endTime := s.formatSRTTime(sub.EndTime)
		content.WriteString(fmt.Sprintf("%s --> %s\n", startTime, endTime))
		if useTranslated {
			content.WriteString(sub.TranslatedText)
		} else {
			content.WriteString(sub.OriginalText)
		}
		content.WriteString("\n\n")
	}
	return os.WriteFile(path, []byte(content.String()), 0644)
}

func (s *DeepseekTranslateStep) formatSRTTime(seconds float64) string {
	hours := int(seconds / 3600)
	minutes := int((seconds - float64(hours*3600)) / 60)
	secs := int(seconds - float64(hours*3600) - float64(minutes*60))
	milliseconds := int((seconds - float64(int(seconds))) * 1000)
	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, secs, milliseconds)
}

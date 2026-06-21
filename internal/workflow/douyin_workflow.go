package workflow

import (
	"context"
	"strings"

	"github.com/zolagz/ytb2bili/internal/config"
	"github.com/zolagz/ytb2bili/internal/service"
	"github.com/zolagz/ytb2bili/pkg/llm"
	"github.com/zolagz/ytb2bili/pkg/tikhub"
	"github.com/zolagz/ytb2bili/pkg/tools"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var DouyinWorkflowModule = fx.Module("douyin_workflow",
	fx.Provide(provideDouyinFetchVideoTool),
	fx.Provide(provideDouyinDownloadVideoTool),
	fx.Options(StepProvidersForGroup("douyin_steps",
		NewInitStep,
		NewResolveDouyinShareStep,
		NewDownloadDouyinVideoStep,
		NewExtractAudioStep,
		NewTranscribeStep,
		NewLLMTranslateStep,
		NewGenerateMetadataStep,
		NewSynthesizeSubtitleAudioStep,
		// NewAddWatermarkStep,
		NewSaveDatabaseStep,
	)...),
	fx.Provide(NewDouyinChain),
)

type DouyinChain struct {
	chain        *Chain
	db           *gorm.DB
	userSettings *service.UserSettingsClient
	logger       *zap.Logger
	resolver     *tools.FetchVideoByShareURLTool
	workflowCfg  config.WorkflowConfig
}

type DouyinChainParams struct {
	fx.In
	Steps        []Step `group:"douyin_steps"`
	DB           *gorm.DB
	UserSettings *service.UserSettingsClient `optional:"true"`
	Logger       *zap.Logger
	Resolver     *tools.FetchVideoByShareURLTool
	Cfg          config.WorkflowConfig
}

func NewDouyinChain(params DouyinChainParams) *DouyinChain {
	chain := NewChainFromSteps(params.Steps, params.Logger, "DouyinTaskChain")

	return &DouyinChain{
		chain:        chain,
		db:           params.DB,
		userSettings: params.UserSettings,
		logger:       params.Logger,
		resolver:     params.Resolver,
		workflowCfg:  params.Cfg,
	}
}

func (dc *DouyinChain) defaultVideoContext() *VideoContext {
	sourceLang := dc.workflowCfg.LLMTranslationSourceLang
	if sourceLang == "" {
		sourceLang = "zh-Hans"
	}
	targetLang := dc.workflowCfg.LLMTranslationTargetLang
	if targetLang == "" {
		targetLang = "zh-Hans"
	}

	return &VideoContext{
		Platform: "douyin",
		TranslationConfig: &TranslationConfig{
			SourceLanguage: sourceLang,
			TargetLanguage: targetLang,
			ModelName:      llm.DefaultTranslationModel,
		},
		SpeechSynthesisConfig: &SpeechSynthesisConfig{
			Language:  "zh-CN",
			VoiceName: "zh-CN-XiaoxiaoNeural",
			Format:    "mp3",
		},
		TaskChainSettings: DefaultTaskChainSettings(),
	}
}

func (dc *DouyinChain) ResolveShareInfo(ctx context.Context, shareURL string) (*tools.DouyinVideoInfo, error) {
	return dc.resolver.Fetch(ctx, shareURL)
}

func (dc *DouyinChain) ProcessContextWithTracking(ctx context.Context, initialCtx *VideoContext, videoID, userID string) (*VideoContext, error) {
	if initialCtx == nil {
		initialCtx = dc.defaultVideoContext()
	}
	initialCtx.Platform = "douyin"
	initialCtx.TaskChainSettings = NormalizeTaskChainSettings(initialCtx.TaskChainSettings)
	resolvedUserID := strings.TrimSpace(userID)
	if resolvedUserID == "" && initialCtx.UserID != "" {
		resolvedUserID = initialCtx.UserID
	}
	ctx = WithVideoID(ctx, videoID)
	if resolvedUserID != "" {
		ctx = WithUserID(ctx, resolvedUserID)
		initialCtx.UserID = resolvedUserID
	}
	applyLatestUserSettingsToVideoContext(ctx, dc.userSettings, dc.logger, initialCtx)
	ctx = withPreferencesApplied(ctx)

	output, err := RunChainWithTracking(ctx, dc.chain, dc.db, dc.logger, videoID, initialCtx)
	if err != nil {
		return nil, err
	}
	output.Platform = "douyin"
	return output, nil
}

func provideDouyinFetchVideoTool(logger *zap.Logger) *tools.FetchVideoByShareURLTool {
	return tools.NewFetchVideoByShareURLTool(tikhub.NewDirectResolver(logger), logger)
}

func provideDouyinDownloadVideoTool(appCfg *config.AppConfig, logger *zap.Logger) (*tools.DownloadDouyinVideoTool, error) {
	return tools.NewDownloadDouyinVideoToolFromAppConfig(appCfg, logger)
}

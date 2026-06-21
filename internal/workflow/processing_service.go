package workflow

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/zolagz/ytb2bili/internal/config"
	"github.com/zolagz/ytb2bili/internal/service"
	"github.com/zolagz/ytb2bili/pkg/llm"
	"github.com/zolagz/ytb2bili/pkg/store/model"
	"github.com/zolagz/ytb2bili/pkg/tools"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ProcessingService struct {
	db           *gorm.DB
	youtubeChain *YouTubeChain
	douyinChain  *DouyinChain
	biliChain    *BilibiliChain
	taskRuntime  *TaskRuntimeRegistry
	logger       *zap.Logger
	cfg          *config.AppConfig
	userSettings *service.UserSettingsClient
	workerSem    chan struct{}
	pipeline     *Pipeline
}

type ProcessingServiceParams struct {
	fx.In
	DB           *gorm.DB                    `optional:"true"`
	YouTubeChain *YouTubeChain               `optional:"true"`
	DouyinChain  *DouyinChain                `optional:"true"`
	BiliChain    *BilibiliChain              `optional:"true"`
	TaskRuntime  *TaskRuntimeRegistry        `optional:"true"`
	Logger       *zap.Logger
	Cfg          *config.AppConfig           `optional:"true"`
	UserSettings *service.UserSettingsClient `optional:"true"`
}

func (s *ProcessingService) BiliChain() *BilibiliChain   { return s.biliChain }
func (s *ProcessingService) YouTubeChain() *YouTubeChain   { return s.youtubeChain }
func (s *ProcessingService) DouyinChain() *DouyinChain     { return s.douyinChain }

func (s *ProcessingService) CancelTask(videoID string) error {
	if s.taskRuntime == nil || !s.taskRuntime.Cancel(videoID) {
		return fmt.Errorf("当前任务未在后台运行，暂时无法停止")
	}
	return nil
}

func NewProcessingService(params ProcessingServiceParams) *ProcessingService {
	maxConcurrent := params.Cfg.Workflow.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 4
	}
	svc := &ProcessingService{
		db:           params.DB,
		youtubeChain: params.YouTubeChain,
		douyinChain:  params.DouyinChain,
		biliChain:    params.BiliChain,
		taskRuntime:  params.TaskRuntime,
		logger:       params.Logger,
		cfg:          params.Cfg,
		userSettings: params.UserSettings,
		workerSem:    make(chan struct{}, maxConcurrent),
	}
	stages := NewStandardStages(
		func(ctx context.Context, task *PipelineTask) error { return nil },
		func(ctx context.Context, task *PipelineTask) error { return nil },
		func(ctx context.Context, task *PipelineTask) error { return nil },
		func(ctx context.Context, task *PipelineTask) error { return nil },
		func(ctx context.Context, task *PipelineTask) error { return nil },
		func(ctx context.Context, task *PipelineTask) error { return nil },
		func(ctx context.Context, task *PipelineTask) error { return nil },
	)
	svc.pipeline = NewPipeline(stages, params.Logger)
	svc.pipeline.Start()
	return svc
}

func (s *ProcessingService) SubmitToPipeline(ctx context.Context, platform string, task *PipelineTask) (<-chan PipelineEvent, error) {
	var chain *Chain
	switch platform {
	case "douyin":
		if s.douyinChain == nil {
			return nil, fmt.Errorf("douyin pipeline not available")
		}
		chain = s.youtubeChain.getChain()
	case "youtube":
		chain = s.youtubeChain.getChain()
	default:
		return nil, fmt.Errorf("unknown platform: %s", platform)
	}
	cp := NewChainPipeline(chain, s.logger)
	cp.Start()
	if task.ID == "" && task.VideoURL != "" {
		p, _, vid, _, err := s.ResolveRemoteVideoTarget(ctx, task.VideoURL)
		if err == nil {
			task.Platform = p
			task.ID = vid
		}
	}
	return cp.Submit(ctx, task)
}

var (
	douyinShareURLPattern = regexp.MustCompile(`https?://(?:v\.douyin\.com|www\.douyin\.com|iesdouyin\.com)[^\s"'<>）】】，,。；;]*`)
	douyinVideoIDPattern  = regexp.MustCompile(`(?:douyin\.com/video/|iesdouyin\.com/share/video/)(\d{10,})`)
	youtubeVideoIDPattern = regexp.MustCompile(`(?:v=|youtu\.be/|shorts/|embed/)([A-Za-z0-9_-]{11})`)
)

func DetectPlatform(raw string) string {
	if youtubeVideoIDPattern.MatchString(raw) {
		return "youtube"
	}
	if douyinShareURLPattern.MatchString(raw) || douyinVideoIDPattern.MatchString(raw) {
		return "douyin"
	}
	return ""
}

func (s *ProcessingService) ResolveRemoteVideoTarget(ctx context.Context, raw string) (string, string, string, *tools.DouyinVideoInfo, error) {
	trimmed := strings.TrimSpace(raw)
	platform := DetectPlatform(trimmed)
	switch platform {
	case "youtube":
		videoID := extractYouTubeID(trimmed)
		if videoID == "" {
			return "", "", "", nil, fmt.Errorf("无法从 YouTube 链接中提取视频 ID")
		}
		return platform, trimmed, videoID, nil, nil
	case "douyin":
		if s.douyinChain == nil {
			return "", "", "", nil, fmt.Errorf("抖音处理链未注册")
		}
		shareInput := trimmed
		if shareURL := extractDouyinShareURL(trimmed); shareURL != "" {
			shareInput = shareURL
		}
		info, err := s.douyinChain.ResolveShareInfo(ctx, shareInput)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("解析抖音分享链接失败: %w", err)
		}
		videoID := strings.TrimSpace(info.Data.AwemeID)
		if videoID == "" {
			videoID = extractDouyinID(shareInput)
		}
		if videoID == "" {
			return "", "", "", nil, fmt.Errorf("无法从抖音分享链接中提取视频 ID")
		}
		if shareURL := strings.TrimSpace(info.Data.ShareURL); shareURL != "" {
			shareInput = shareURL
		}
		return platform, shareInput, videoID, info, nil
	default:
		return "", "", "", nil, fmt.Errorf("暂不支持该链接，当前仅支持 YouTube 和抖音分享链接")
	}
}

func (s *ProcessingService) ProcessRemoteVideo(ctx context.Context, platform, normalizedURL, videoID, userID, preferredResolution string,
	douyinInfo *tools.DouyinVideoInfo, taskChainSettings *TaskChainSettings, speechConfig *SpeechSynthesisConfig) (*VideoContext, error) {
	translationConfig := s.resolveTranslationConfig(ctx, userID)
	initialCtx := &VideoContext{
		Platform: platform, VideoURL: normalizedURL, VideoID: videoID, UserID: userID,
		DouyinVideoInfo: douyinInfo, TranslationConfig: translationConfig,
		SpeechSynthesisConfig: speechConfig, TaskChainSettings: taskChainSettings,
	}
	if platform == "douyin" {
		return s.douyinChain.ProcessContextWithTracking(ctx, initialCtx, videoID, userID)
	}
	return s.youtubeChain.ProcessContextWithTracking(ctx, initialCtx, videoID, userID)
}

func (s *ProcessingService) EnqueueRemoteVideoProcessing(platform, normalizedURL, videoID, userID, preferredResolution string,
	douyinInfo *tools.DouyinVideoInfo, taskChainSettings *TaskChainSettings, speechConfig *SpeechSynthesisConfig) {
	s.workerSem <- struct{}{}
	go func() {
		defer func() { <-s.workerSem }()
		baseCtx := context.Background()
		ctx, cancel := context.WithTimeout(baseCtx, 30*time.Minute)
		if s.taskRuntime != nil {
			s.taskRuntime.Register(videoID, cancel)
			defer s.taskRuntime.Finish(videoID)
		}
		defer cancel()
		_, procErr := s.ProcessRemoteVideo(ctx, platform, normalizedURL, videoID, userID, preferredResolution,
			douyinInfo, taskChainSettings, speechConfig)
		if procErr != nil {
			if errors.Is(procErr, context.Canceled) {
				MarkVideoStopped(s.db, s.logger, videoID)
				s.db.Model(&model.Video{}).Where("video_id = ?", videoID).Update("status", model.VideoStatusPaused)
				return
			}
			s.logger.Error("AsyncSubmitLink: 视频处理失败", zap.String("platform", platform), zap.String("video_id", videoID), zap.Error(procErr))
			s.db.Model(&model.Video{}).Where("video_id = ?", videoID).Update("status", model.VideoStatusFailed)
			return
		}
		s.logger.Info("AsyncSubmitLink: 视频处理完成", zap.String("platform", platform), zap.String("video_id", videoID))
	}()
}

func (s *ProcessingService) resolveTranslationConfig(ctx context.Context, userID string) *TranslationConfig {
	sourceLang := s.cfg.Workflow.LLMTranslationSourceLang
	if sourceLang == "" {
		sourceLang = "en"
	}
	targetLang := s.cfg.Workflow.LLMTranslationTargetLang
	if targetLang == "" {
		targetLang = "zh-Hans"
	}
	tc := &TranslationConfig{SourceLanguage: sourceLang, TargetLanguage: targetLang, ModelName: llm.DefaultTranslationModel}
	if s.userSettings != nil && s.userSettings.IsEnabled() && strings.TrimSpace(userID) != "" {
		settings, err := s.userSettings.GetSettings(ctx, userID)
		if err == nil {
			if v := strings.TrimSpace(settings["translation_source_lang"]); v != "" {
				tc.SourceLanguage = v
			}
			if v := strings.TrimSpace(settings["translation_target_lang"]); v != "" {
				tc.TargetLanguage = v
			}
			if v := strings.TrimSpace(settings["translation_model"]); v != "" {
				tc.ModelName = v
			}
		}
	}
	return tc
}

func normalizeResolution(resolution string) string {
	switch strings.TrimSpace(resolution) {
	case "best", "720p", "1080p", "1440p", "2160p", "4k":
		return strings.TrimSpace(resolution)
	default:
		return "best"
	}
}

func extractYouTubeID(rawURL string) string {
	matches := youtubeVideoIDPattern.FindStringSubmatch(rawURL)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func extractDouyinShareURL(raw string) string {
	matches := douyinShareURLPattern.FindString(raw)
	return strings.TrimRight(matches, ",，。；;!！?？)）]】>》\"")
}

func extractDouyinID(raw string) string {
	matches := douyinVideoIDPattern.FindStringSubmatch(raw)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

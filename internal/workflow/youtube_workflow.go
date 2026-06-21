package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/zolagz/ytb2bili/internal/config"
	"github.com/zolagz/ytb2bili/internal/service"
	"github.com/zolagz/ytb2bili/pkg/llm"
	"github.com/zolagz/ytb2bili/pkg/store/model"
	"github.com/zolagz/ytb2bili/pkg/tools"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// YouTubeWorkflowModule 提供 YouTube 视频处理工作流的所有组件
var YouTubeWorkflowModule = fx.Module("youtube_workflow",
	// 提供配置（从 AppConfig 中提取）
	fx.Provide(provideWorkflowConfig),
	fx.Provide(NewTaskRuntimeRegistry),

	// 提供工具
	fx.Provide(provideDownloadVideoTool),
	fx.Provide(provideDownloadThumbnailTool),
	fx.Provide(provideExtractAudioTool),
	fx.Provide(provideTranscriberTool),
	fx.Provide(provideLLMBatchTranslatorTool),
	fx.Provide(provideTTSClientTool),

	// 提供任务链
	fx.Provide(NewChain),

	// 通过 StepProvidersForGroup 批量注册各步骤，新增步骤只需追加一行构造函数
	fx.Options(StepProvidersForGroup("steps",
		NewInitStep,
		NewDownloadVideoStep,
		NewDownloadThumbnailStep,
		NewExtractAudioStep,
		NewTranscribeStep,
		//NewDeepseekTranslateStep,     // 使用 Deepseek LLM 翻译，以 LLMTranslate 名义对前端展示
	    NewLLMTranslateStep,
		NewGenerateMetadataStep,
		NewSynthesizeSubtitleAudioStep,
		// NewAddWatermarkStep,
		NewSaveDatabaseStep,
	)...),

	// 提供 YouTube 处理链
	fx.Provide(NewYouTubeChain),
)

// ============================================================================
// Context 定义
// ============================================================================

// SubtitleAudio 字幕音频信息
type SubtitleAudio struct {
	OriginalText   string  // 原始字幕文本
	TranslatedText string  // 翻译后的文本
	AudioPath      string  // 合成的音频文件路径
	StartTime      float64 // 字幕开始时间
	EndTime        float64 // 字幕结束时间
}

// TranslationConfig 翻译配置
type TranslationConfig struct {
	SourceLanguage string // 源语言，默认 "en"
	TargetLanguage string // 目标语言，默认 "zh-Hans"
	ModelName      string // 翻译模型，默认使用 pkg/llm.DefaultTranslationModel
}

// SpeechSynthesisConfig 语音合成配置
type SpeechSynthesisConfig struct {
	Language  string  `json:"language"`           // 语言代码，如 "zh-CN"
	VoiceName string  `json:"voice_name"`         // 语音名称，如 "zh-CN-XiaoxiaoNeural"
	Format    string  `json:"format"`             // 音频格式，如 "mp3"
	Provider  string  `json:"provider,omitempty"` // 指定 TTS 提供商，如 azure/tencent/auto
	Search    string  `json:"search,omitempty"`   // 音色搜索关键词
	Rate      float64 `json:"rate,omitempty"`     // 语速
	Volume    float64 `json:"volume,omitempty"`   // 音量
	Pitch     float64 `json:"pitch,omitempty"`    // 音高
}

func (c *SpeechSynthesisConfig) GetLanguage() string {
	if c == nil {
		return ""
	}
	return c.Language
}

func (c *SpeechSynthesisConfig) GetVoiceName() string {
	if c == nil {
		return ""
	}
	return c.VoiceName
}

func (c *SpeechSynthesisConfig) GetFormat() string {
	if c == nil {
		return ""
	}
	return c.Format
}

func (c *SpeechSynthesisConfig) GetProvider() string {
	if c == nil {
		return ""
	}
	return c.Provider
}

func (c *SpeechSynthesisConfig) GetSearch() string {
	if c == nil {
		return ""
	}
	return c.Search
}

func (c *SpeechSynthesisConfig) GetRate() float64 {
	if c == nil {
		return 0
	}
	return c.Rate
}

func (c *SpeechSynthesisConfig) GetVolume() float64 {
	if c == nil {
		return 0
	}
	return c.Volume
}

func (c *SpeechSynthesisConfig) GetPitch() float64 {
	if c == nil {
		return 0
	}
	return c.Pitch
}

type TaskChainSettings struct {
	DownloadThumbnail       bool `json:"download_thumbnail"`
	Transcribe              bool `json:"transcribe"`
	TranslateSubtitles      bool `json:"translate_subtitles"`
	SynthesizeSubtitleAudio bool `json:"synthesize_subtitle_audio"`
}

func DefaultTaskChainSettings() *TaskChainSettings {
	return &TaskChainSettings{
		DownloadThumbnail:       true,
		Transcribe:              true,
		TranslateSubtitles:      true,
		SynthesizeSubtitleAudio: true,
	}
}

func NormalizeTaskChainSettings(settings *TaskChainSettings) *TaskChainSettings {
	normalized := DefaultTaskChainSettings()
	if settings == nil {
		return normalized
	}

	normalized.DownloadThumbnail = settings.DownloadThumbnail
	normalized.Transcribe = settings.Transcribe
	normalized.TranslateSubtitles = settings.TranslateSubtitles
	normalized.SynthesizeSubtitleAudio = settings.SynthesizeSubtitleAudio

	return normalized
}

// VideoContext 视频处理的上下文，在步骤间传递
type VideoContext struct {
	Platform            string
	VideoURL            string
	VideoID             string
	UserID              string // 用户ID（用于计费等）
	PreferredResolution string // 期望下载分辨率: best/720p/1080p/1440p/2160p
	VideoPath           string
	ThumbnailPath       string
	AudioPath           string
	DouyinVideoInfo     *tools.DouyinVideoInfo
	Transcript          *tools.TranscriptResult
	SubtitleAudios      []SubtitleAudio // 每条字幕对应的音频信息

	// 生成的元数据字段
	Title       string // 生成的视频标题
	Description string // 生成的视频描述
	Tags        string // 生成的视频标签（逗号分隔）

	// B站上传相关字段
	BiliBVID     string // B站视频BVID
	BiliAID      int64  // B站视频AID
	BiliAccountID uint  // 指定投稿使用的 B 站账号绑定 ID

	// 可配置参数（可通过接口设置）
	TranslationConfig     *TranslationConfig     // 翻译配置
	SpeechSynthesisConfig *SpeechSynthesisConfig // 语音合成配置
	TaskChainSettings     *TaskChainSettings     // 任务链步骤开关
	RestartFromStep       string                 // 指定续跑起点；起点之前的步骤在运行时严格跳过
	TranslationSkipped    bool                   // 当前字幕是否判定为无需翻译
	restartStepActivated  bool
}

// ============================================================================
// YouTube 任务链
// ============================================================================

type YouTubeChain struct {
	chain        *Chain
	db           *gorm.DB
	userSettings *service.UserSettingsClient
	logger       *zap.Logger
	downloadDir  string
	workflowCfg  config.WorkflowConfig
}

type YouTubeChainParams struct {
	fx.In
	Chain        *Chain
	DB           *gorm.DB
	UserSettings *service.UserSettingsClient `optional:"true"`
	Logger       *zap.Logger
	Cfg          config.WorkflowConfig
}


func (yc *YouTubeChain) getChain() *Chain { return yc.chain }
func NewYouTubeChain(params YouTubeChainParams) *YouTubeChain {
	return &YouTubeChain{
		chain:        params.Chain,
		db:           params.DB,
		userSettings: params.UserSettings,
		logger:       params.Logger,
		downloadDir:  params.Cfg.DownloadDir,
		workflowCfg:  params.Cfg,
	}
}


func (yc *YouTubeChain) defaultVideoContext() *VideoContext {
	sourceLang := yc.workflowCfg.LLMTranslationSourceLang
	if sourceLang == "" {
		sourceLang = "en"
	}
	targetLang := yc.workflowCfg.LLMTranslationTargetLang
	if targetLang == "" {
		targetLang = "zh-Hans"
	}

	return &VideoContext{
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

// Process 处理视频（无追踪，向后兼容）
func (yc *YouTubeChain) Process(ctx context.Context, videoURL string) (*VideoContext, error) {
	result := yc.chain.Run(ctx, videoURL)

	if !result.Success {
		return nil, result.Error
	}

	vctx, ok := result.FinalOutput.(*VideoContext)
	if !ok {
		return nil, fmt.Errorf("unexpected output type: %T", result.FinalOutput)
	}

	return vctx, nil
}

// ProcessWithTracking 处理视频并将步骤进度持久化到数据库
// userID 可传空字符串，传入则开启每步积分扣减
func (yc *YouTubeChain) ProcessWithTracking(ctx context.Context, videoURL, videoID, userID string) (*VideoContext, error) {
	initialCtx := yc.defaultVideoContext()
	initialCtx.VideoURL = videoURL
	return yc.ProcessContextWithTracking(ctx, initialCtx, videoID, userID)
}

func (yc *YouTubeChain) ProcessContextWithTracking(ctx context.Context, initialCtx *VideoContext, videoID, userID string) (*VideoContext, error) {
	if initialCtx == nil {
		initialCtx = yc.defaultVideoContext()
	}
	resolvedUserID := yc.resolveUserIDForRun(ctx, videoID, userID, initialCtx)
	ctx = WithVideoID(ctx, videoID)
	if resolvedUserID != "" {
		ctx = WithUserID(ctx, resolvedUserID)
		initialCtx.UserID = resolvedUserID
	}
	applyLatestUserSettingsToVideoContext(ctx, yc.userSettings, yc.logger, initialCtx)
	ctx = withPreferencesApplied(ctx)

	return RunChainWithTracking(ctx, yc.chain, yc.db, yc.logger, videoID, initialCtx)
}

// ProcessLocalWithTracking 处理本地视频文件并将步骤进度持久化到数据库
// 与 ProcessWithTracking 相比，会自动跳过 Init/DownloadVideo/DownloadThumbnail 步骤
// userID 可传空字符串，传入则开启每步积分扣减
func (yc *YouTubeChain) ProcessLocalWithTracking(ctx context.Context, videoPath, videoID, title, userID string) (*VideoContext, error) {
	initialCtx := yc.defaultVideoContext()
	initialCtx.VideoID = videoID
	initialCtx.VideoPath = videoPath
	initialCtx.Title = title
	return yc.ProcessLocalContextWithTracking(ctx, initialCtx, videoID, userID)
}

func (yc *YouTubeChain) ProcessLocalContextWithTracking(ctx context.Context, initialCtx *VideoContext, videoID, userID string) (*VideoContext, error) {
	// 传入预填充的 VideoContext，InitStep 会透传，DownloadVideo/DownloadThumbnail 会自动跳过
	if initialCtx == nil {
		initialCtx = yc.defaultVideoContext()
	}
	resolvedUserID := yc.resolveUserIDForRun(ctx, videoID, userID, initialCtx)
	ctx = WithVideoID(ctx, videoID)
	if resolvedUserID != "" {
		ctx = WithUserID(ctx, resolvedUserID)
		initialCtx.UserID = resolvedUserID
	}
	applyLatestUserSettingsToVideoContext(ctx, yc.userSettings, yc.logger, initialCtx)
	ctx = withPreferencesApplied(ctx)

	return RunChainWithTracking(ctx, yc.chain, yc.db, yc.logger, videoID, initialCtx)
}

func (yc *YouTubeChain) resolveUserIDForRun(ctx context.Context, videoID, requestedUserID string, initialCtx *VideoContext) string {
	if resolved := strings.TrimSpace(requestedUserID); resolved != "" {
		return resolved
	}
	if initialCtx != nil {
		if resolved := strings.TrimSpace(initialCtx.UserID); resolved != "" {
			return resolved
		}
	}
	if resolved := strings.TrimSpace(GetUserID(ctx)); resolved != "" {
		return resolved
	}
	if strings.TrimSpace(videoID) == "" || yc.db == nil {
		return ""
	}

	var video model.Video
	if err := yc.db.WithContext(ctx).Select("user_id").Where("video_id = ?", videoID).First(&video).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			yc.logger.Warn("Failed to resolve user ID from video record",
				zap.String("video_id", videoID),
				zap.Error(err))
		}
		return ""
	}

	resolved := strings.TrimSpace(video.UserID)
	if resolved != "" {
		yc.logger.Info("Resolved workflow user ID from video record",
			zap.String("video_id", videoID),
			zap.String("user_id", resolved))
	}
	return resolved
}

// RetryStepByName 直接重试指定名称的步骤，从数据库与磁盘重建上下文，不触发完整流程
func (yc *YouTubeChain) RetryStepByName(ctx context.Context, video *model.Video, stepName string) error {
	// 在链中按名称查找步骤
	var targetStep Step
	for _, s := range yc.chain.GetSteps() {
		if s.Name() == stepName {
			targetStep = s
			break
		}
	}
	if targetStep == nil {
		return fmt.Errorf("步骤 %q 在链中不存在", stepName)
	}

	// 解析有效的视频 URL（video.URL 可能因历史原因存储的是本地文件路径）
	videoURL := resolveVideoURL(video.URL, video.VideoPath)

	// 从 DB 重建最小可用的 VideoContext
	vctx := yc.defaultVideoContext()
	vctx.VideoID = video.VideoID
	vctx.VideoURL = videoURL
	vctx.VideoPath = video.VideoPath

	// 对于「下载视频」步骤的特殊处理
	if stepName == StepNameDownloadVideo {
		if video.VideoPath != "" {
			// 若本地文件仍然存在，直接复用，无需重新下载
			if _, statErr := os.Stat(video.VideoPath); statErr == nil {
				yc.logger.Info("本地视频文件已存在，跳过重新下载",
					zap.String("video_id", video.VideoID),
					zap.String("path", video.VideoPath))
				tracker := NewProgressTracker(yc.db, yc.logger)
				ctx = WithVideoID(ctx, video.VideoID)
				tracker.BeforeStep(video.VideoID, stepName)
				tracker.AfterStep(video.VideoID, stepName, model.TaskStepStatusCompleted, "")
				return nil
			}
		}
		// 文件不存在，需要重新下载
		if videoURL == "" {
			return fmt.Errorf("无法获取视频 URL，请先确认原始 YouTube 地址已录入")
		}
		vctx.VideoPath = "" // 清空，让步骤重新下载
		vctx.VideoID = ""
	}

	yc.restoreTranscriptFromSavedSubtitles(video, vctx)

	tracker := NewProgressTracker(yc.db, yc.logger)
	ctx = WithVideoID(ctx, video.VideoID)
	// 将用户ID注入context，供上传到B站等需要鉴权的步骤使用
	if video.UserID != "" {
		ctx = WithUserID(ctx, video.UserID)
		vctx.UserID = video.UserID
	}
	applyLatestUserSettingsToVideoContext(ctx, yc.userSettings, yc.logger, vctx)

	tracker.BeforeStep(video.VideoID, stepName)
	_, err := targetStep.Execute(ctx, vctx)
	if err != nil {
		tracker.AfterStep(video.VideoID, stepName, model.TaskStepStatusFailed, err.Error())
		return err
	}
	tracker.AfterStep(video.VideoID, stepName, model.TaskStepStatusCompleted, "")
	return nil
}

// ResumeProcessing 从上次中断处续跑视频处理链。
// 已完成的步骤（completed/skipped）会保留；其余步骤重置為 pending 后重新执行。
// 如果 video.VideoPath 为空，会尝试在 downloadDir/{videoID}/ 目录中自动查找视频文件。
func (yc *YouTubeChain) ResumeProcessing(ctx context.Context, video *model.Video) error {
	return yc.resumeProcessing(ctx, video, nil, true)
}

func (yc *YouTubeChain) ResumeProcessingWithContext(ctx context.Context, video *model.Video, initialCtx *VideoContext) error {
	return yc.resumeProcessing(ctx, video, initialCtx, false)
}

func (yc *YouTubeChain) resumeProcessing(ctx context.Context, video *model.Video, initialCtx *VideoContext, refreshUserSettings bool) error {
	videoPath := video.VideoPath
	if videoPath == "" {
		videoPath = yc.findLocalVideoFile(video.VideoID)
	}

	ctx = WithVideoID(ctx, video.VideoID)
	if video.UserID != "" {
		ctx = WithUserID(ctx, video.UserID)
	}

	videoURL := resolveVideoURL(video.URL, videoPath)

	if initialCtx == nil {
		initialCtx = yc.defaultVideoContext()
	}
	initialCtx.VideoID = video.VideoID
	initialCtx.VideoPath = videoPath
	initialCtx.VideoURL = videoURL
	if strings.TrimSpace(initialCtx.Title) == "" {
		initialCtx.Title = video.Title
	}
	initialCtx.UserID = video.UserID
	if refreshUserSettings {
		applyLatestUserSettingsToVideoContext(ctx, yc.userSettings, yc.logger, initialCtx)
	}
	yc.restoreTranscriptFromSavedSubtitles(video, initialCtx)
	yc.restoreSubtitleAudiosFromSavedSubtitles(initialCtx)

	yc.logger.Info("续跑视频处理链",
		zap.String("video_id", video.VideoID),
		zap.String("video_path", videoPath),
		zap.String("video_url", videoURL))

	_, err := RunChainWithTracking(ctx, yc.chain, yc.db, yc.logger, video.VideoID, initialCtx)
	return err
}

func (yc *YouTubeChain) restoreTranscriptFromSavedSubtitles(video *model.Video, vctx *VideoContext) {
	if yc == nil || video == nil || vctx == nil || vctx.Transcript != nil {
		return
	}

	for _, srtPath := range yc.transcriptSubtitleCandidates(video, vctx) {
		data, err := os.ReadFile(srtPath)
		if err != nil {
			continue
		}
		entries, parseErr := tools.ParseSRTContent(string(data))
		if parseErr != nil || len(entries) == 0 {
			continue
		}

		vctx.Transcript = tools.SRTEntriesToTranscript(entries, srtPath)
		if yc.logger != nil {
			yc.logger.Info("已从SRT文件重建转录结果",
				zap.String("video_id", strings.TrimSpace(video.VideoID)),
				zap.String("srt_path", srtPath),
				zap.Int("segments", len(vctx.Transcript.Segments)))
		}
		return
	}

	if yc.logger != nil {
		yc.logger.Warn("未找到可用于恢复转录结果的字幕文件",
			zap.String("video_id", strings.TrimSpace(video.VideoID)),
			zap.String("subtitle_path", strings.TrimSpace(video.SubtitlePath)),
			zap.String("video_path", strings.TrimSpace(vctx.VideoPath)))
	}
}

func (yc *YouTubeChain) transcriptSubtitleCandidates(video *model.Video, vctx *VideoContext) []string {
	if video == nil || vctx == nil {
		return nil
	}

	videoID := strings.TrimSpace(video.VideoID)
	videoBaseName := strings.TrimSuffix(filepath.Base(strings.TrimSpace(vctx.VideoPath)), filepath.Ext(strings.TrimSpace(vctx.VideoPath)))
	if videoBaseName == "" {
		videoBaseName = strings.TrimSuffix(filepath.Base(strings.TrimSpace(video.VideoPath)), filepath.Ext(strings.TrimSpace(video.VideoPath)))
	}

	type scoredPath struct {
		path  string
		score int
	}

	seen := make(map[string]struct{})
	var candidates []scoredPath
	addCandidate := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		info, err := os.Stat(path)
		if err != nil || info.IsDir() || info.Size() == 0 {
			return
		}
		seen[path] = struct{}{}
		candidates = append(candidates, scoredPath{path: path, score: scoreTranscriptSubtitlePath(path, videoID, videoBaseName)})
	}

	var candidateDirs []string
	addDir := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		for _, existing := range candidateDirs {
			if existing == path {
				return
			}
		}
		candidateDirs = append(candidateDirs, path)
	}

	addCandidate(video.SubtitlePath)
	addDir(filepath.Dir(strings.TrimSpace(video.SubtitlePath)))
	addDir(filepath.Dir(strings.TrimSpace(vctx.VideoPath)))
	addDir(filepath.Dir(strings.TrimSpace(video.VideoPath)))
	if yc.downloadDir != "" && videoID != "" {
		addDir(filepath.Join(yc.downloadDir, videoID))
	}

	for _, dir := range candidateDirs {
		if dir == "." || dir == "" {
			continue
		}
		if videoID != "" {
			addCandidate(filepath.Join(dir, videoID+".en.srt"))
			addCandidate(filepath.Join(dir, videoID+".srt"))
		}
		if videoBaseName != "" {
			addCandidate(filepath.Join(dir, videoBaseName+".en.srt"))
			addCandidate(filepath.Join(dir, videoBaseName+".srt"))
		}
		addCandidate(filepath.Join(dir, "en.srt"))
		addCandidate(filepath.Join(dir, "subtitle.srt"))
		addCandidate(filepath.Join(dir, "subtitles.srt"))

		matches, _ := filepath.Glob(filepath.Join(dir, "*.srt"))
		for _, match := range matches {
			addCandidate(match)
		}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].path < candidates[j].path
		}
		return candidates[i].score > candidates[j].score
	})

	paths := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		paths = append(paths, candidate.path)
	}
	return paths
}

func scoreTranscriptSubtitlePath(path, videoID, videoBaseName string) int {
	name := strings.ToLower(filepath.Base(path))
	score := 0

	if name == "en.srt" {
		score += 10000
	}
	if strings.HasSuffix(name, ".en.srt") || strings.Contains(name, ".en-") || strings.Contains(name, ".en_") {
		score += 9500
	}
	if name == "subtitle.srt" || name == "subtitles.srt" {
		score += 100
	}

	if videoID != "" {
		vid := strings.ToLower(videoID)
		if name == vid+".en.srt" {
			score += 10000
		}
		if name == vid+".srt" {
			score += 8000
		}
		if strings.Contains(name, vid) {
			score += 200
		}
	}
	if videoBaseName != "" {
		baseName := strings.ToLower(videoBaseName)
		if name == baseName+".en.srt" {
			score += 9000
		}
		if name == baseName+".srt" {
			score += 7000
		}
		if strings.Contains(name, baseName) {
			score += 100
		}
	}

	if name == "zh.srt" || strings.HasSuffix(name, ".zh.srt") || strings.Contains(name, ".zh-") || strings.Contains(name, ".zh_") || strings.Contains(name, "zh-cn") || strings.Contains(name, "zh_hans") || strings.Contains(name, "zh-hans") {
		score -= 20000
	}

	if strings.HasSuffix(name, ".srt") {
		score += 50
	}

	return score
}

func (yc *YouTubeChain) restoreSubtitleAudiosFromSavedSubtitles(vctx *VideoContext) {
	if vctx == nil || len(vctx.SubtitleAudios) > 0 {
		return
	}
	settings := NormalizeTaskChainSettings(vctx.TaskChainSettings)
	if !settings.SynthesizeSubtitleAudio {
		return
	}
	videoPath := strings.TrimSpace(vctx.VideoPath)
	videoID := strings.TrimSpace(vctx.VideoID)
	if videoPath == "" || videoID == "" {
		return
	}

	videoDir := filepath.Dir(videoPath)
	candidates := []string{
		filepath.Join(videoDir, videoID+".zh.srt"),
		filepath.Join(videoDir, videoID+".srt"),
		filepath.Join(videoDir, "zh.srt"),
	}

	for _, candidate := range candidates {
		raw, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		entries, err := tools.ParseSRTContent(string(raw))
		if err != nil || len(entries) == 0 {
			continue
		}

		subtitles := make([]SubtitleAudio, 0, len(entries))
		for _, entry := range entries {
			text := strings.TrimSpace(entry.Text)
			if text == "" {
				continue
			}
			start, end := tools.ParseSRTTimeCode(entry.TimeCode)
			subtitles = append(subtitles, SubtitleAudio{
				OriginalText:   text,
				TranslatedText: text,
				StartTime:      start,
				EndTime:        end,
			})
		}
		if len(subtitles) == 0 {
			continue
		}

		vctx.SubtitleAudios = subtitles
		yc.logger.Info("已从字幕文件重建配音输入",
			zap.String("video_id", videoID),
			zap.String("subtitle_path", candidate),
			zap.Int("count", len(subtitles)))
		return
	}
	if yc.logger != nil {
		yc.logger.Warn("未找到可用于重新配音的字幕文件",
			zap.String("video_id", videoID),
			zap.String("video_path", videoPath))
	}
}

// findLocalVideoFile 在 downloadDir/{videoID}/ 中查找已下载的视频文件。
func (yc *YouTubeChain) findLocalVideoFile(videoID string) string {
	if yc.downloadDir == "" || videoID == "" {
		return ""
	}
	// 先按 yt-dlp 默认命名约定精确查找
	for _, ext := range []string{"mp4", "mkv", "webm", "flv", "avi"} {
		p := filepath.Join(yc.downloadDir, videoID, videoID+"."+ext)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// Glob 兜底：目录下任意视频文件
	matches, _ := filepath.Glob(filepath.Join(yc.downloadDir, videoID, "*"))
	for _, m := range matches {
		switch strings.ToLower(filepath.Ext(m)) {
		case ".mp4", ".mkv", ".webm", ".flv", ".avi":
			return m
		}
	}
	return ""
}

// resolveVideoURL 返回一个可用于下载的 HTTP(S) URL。
// 若 storedURL 已是合法 HTTP URL 则直接使用；
// 否则尝试由 videoPath 文件名（yt-dlp 按 %(id)s.%(ext)s 命名）重建 YouTube URL。
func resolveVideoURL(storedURL, videoPath string) string {
	if strings.HasPrefix(storedURL, "http://") || strings.HasPrefix(storedURL, "https://") {
		return storedURL
	}
	// storedURL 不是 HTTP URL（可能是本地路径或空），从文件名提取 YouTube ID
	if videoPath != "" {
		base := filepath.Base(videoPath)
		ytID := strings.TrimSuffix(base, filepath.Ext(base))
		// YouTube 视频 ID 固定 11 位
		if len(ytID) == 11 && !strings.ContainsAny(ytID, "/\\") {
			return "https://www.youtube.com/watch?v=" + ytID
		}
	}
	return storedURL
}

// ============================================================================
// 配置提供者
// ============================================================================

// provideWorkflowConfig 从 AppConfig 中提取工作流配置
func provideWorkflowConfig(appCfg *config.AppConfig) config.WorkflowConfig {
	return appCfg.Workflow
}

// ============================================================================
// 工具提供者（使用配置统一管理）
// ============================================================================

func provideDownloadVideoTool(cfg config.WorkflowConfig, logger *zap.Logger) (*tools.DownloadVideoTool, error) {
	return tools.NewDownloadVideoTool(tools.DownloadVideoConfig{
		DownloadDir: cfg.DownloadDir,
		CookiesDir:  cfg.CookiesDir,
		YtDlpPath:   cfg.YtDlpPath,
		CookiesFile: cfg.CookiesFile,
		ProxyURL:    cfg.ProxyURL,
	}, logger)
}

func provideDownloadThumbnailTool(appCfg *config.AppConfig, logger *zap.Logger) (*tools.DownloadThumbnailTool, error) {
	return tools.NewDownloadThumbnailToolFromAppConfig(appCfg, logger)
}

func provideExtractAudioTool(appCfg *config.AppConfig, logger *zap.Logger) (*tools.ExtractAudioTool, error) {
	return tools.NewExtractAudioToolFromAppConfig(appCfg, logger)
}

func provideTranscriberTool(logger *zap.Logger) *tools.BcutTranscriberTool {
	return tools.NewBcutTranscriberTool(logger)
}

func provideTranslatorTool(appCfg *config.AppConfig) *tools.MicrosoftTranslator {
	return tools.NewMicrosoftTranslatorFromAppConfig(appCfg)
}

// provideLLMBatchTranslatorTool 提供 LLM 批量字幕翻译工具（统一 BatchTranslator）
func provideLLMBatchTranslatorTool(cfg config.WorkflowConfig, agCfg *config.AppConfig, userSettings *service.UserSettingsClient, logger *zap.Logger) (*tools.BatchTranslator, error) {
	// 如果未启用LLM翻译，返回nil（步骤会被跳过）
	if !cfg.LLMTranslationEnabled {
		logger.Info("LLM batch translation is disabled")
		return nil, nil
	}

	p := agCfg.ResolveTranslationProvider()
	if p == nil || !p.ToLLMConfig().IsValid() {
		logger.Info("LLM subtitle translation not configured (no API key); fallback chat provider will be used")
		p = agCfg.ResolveChatProvider()
	}
	if p == nil || !p.ToLLMConfig().IsValid() {
		logger.Warn("LLM subtitle translation unavailable: no provider configured")
		return nil, nil
	}

	chatLLM, err := llm.NewClientFromConfig(p.ToLLMConfig(), logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM translation client: %w", err)
	}

	translator := tools.NewBatchTranslator(chatLLM, tools.BatchTranslatorConfig{
		SourceLang:  cfg.LLMTranslationSourceLang,
		TargetLang:  cfg.LLMTranslationTargetLang,
		BatchSize:   cfg.LLMTranslationBatchSize,
		MaxWorkers:  cfg.LLMTranslationMaxWorkers,
		ContextSize: cfg.LLMTranslationContextSize,
	}, logger)

	logger.Info("BatchTranslator created",
		zap.String("provider", p.Provider),
		zap.String("model", p.Model),
		zap.String("source_lang", cfg.LLMTranslationSourceLang),
		zap.String("target_lang", cfg.LLMTranslationTargetLang),
		zap.Int("batch_size", cfg.LLMTranslationBatchSize),
		zap.Int("max_workers", cfg.LLMTranslationMaxWorkers))

	return translator, nil
}


// provideTTSClientTool 提供 TTS 客户端工具。
// 是否执行字幕音频合成由用户前端配置的 TaskChainSettings.SynthesizeSubtitleAudio 决定。
func provideTTSClientTool(appCfg *config.AppConfig, db *gorm.DB, logger *zap.Logger) (*tools.TTSClient, error) {
	// 始终创建客户端；具体是否执行由任务配置和用户设置控制。
	ttsClient := tools.NewTTSClient(appCfg, db, logger)

	ttsCfg := appCfg.GetTTSConfig()
	logger.Info("TTS client created",
		zap.String("provider", ttsCfg.Provider),
		zap.String("voice", ttsCfg.Voice),
		zap.String("locale", ttsCfg.Locale))

	return ttsClient, nil
}

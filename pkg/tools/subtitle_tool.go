package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/zolagz/ytb2bili/pkg/llm"
	storemodel "github.com/zolagz/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const subtitleToolName = "subtitle_action"

// SubtitleTool can summarize a video's content or translate its subtitles via LLM.
type SubtitleTool struct {
	db           *gorm.DB
	llmClient    *llm.EinoChatClient
	userSettings UserSettingsProvider
	downloadDir  string
	logger       *zap.Logger
	userID       string // 由 AgentHandler 每次请求前通过 SetUserContext 注入
}

// NewSubtitleTool creates a SubtitleTool.
func NewSubtitleTool(db *gorm.DB, llmClient *llm.EinoChatClient, userSettings UserSettingsProvider, downloadDir string, logger *zap.Logger) *SubtitleTool {
	return &SubtitleTool{db: db, llmClient: llmClient, userSettings: userSettings, downloadDir: downloadDir, logger: logger}
}

// SetUserContext implements ContextualTool — called by AgentHandler before each Run.
func (t *SubtitleTool) SetUserContext(userID string) { t.userID = userID }

func (t *SubtitleTool) resolveLLMClient(ctx context.Context, modelName string) (*llm.EinoChatClient, error) {
	if t.llmClient == nil {
		return nil, fmt.Errorf("LLM client unavailable for subtitle actions")
	}
	return t.llmClient, nil
}

func (t *SubtitleTool) resolveActionModel(ctx context.Context, action string) string {
	defaultModel := llm.DefaultModel
	if strings.EqualFold(strings.TrimSpace(action), "translate") {
		defaultModel = llm.DefaultTranslationModel
	}
	if t.userSettings == nil || !t.userSettings.IsEnabled() || strings.TrimSpace(t.userID) == "" {
		return defaultModel
	}
	settings, err := t.userSettings.GetSettings(ctx, t.userID)
	if err != nil {
		t.logger.Warn("加载用户模型设置失败", zap.String("user_id", t.userID), zap.String("action", action), zap.Error(err))
		return defaultModel
	}
	if strings.EqualFold(strings.TrimSpace(action), "translate") {
		if value := strings.TrimSpace(settings[storemodel.UserSettingKeyTranslationModel]); value != "" {
			return value
		}
	}
	if value := strings.TrimSpace(settings[storemodel.UserSettingKeyPreferredAIModel]); value != "" {
		return value
	}
	return defaultModel
}

// Info describes the tool to the LLM.
func (t *SubtitleTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "subtitle_action",
		Desc: `对已下载视频的字幕执行AI操作：
- summarize: 总结视频内容，返回要点摘要
- translate: 将字幕翻译成指定语言（默认中文）`,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"video_id": {
				Type:     schema.String,
				Desc:     "YouTube视频ID",
				Required: true,
			},
			"action": {
				Type:     schema.String,
				Desc:     "执行动作: summarize(总结) 或 translate(翻译)",
				Required: true,
			},
			"target_lang": {
				Type: schema.String,
				Desc: "翻译目标语言（仅translate时有效），如 zh-CN、en、ja、ko，默认 zh-CN",
			},
		}),
	}, nil
}

// InvokableRun executes the subtitle action.
func (t *SubtitleTool) InvokableRun(ctx context.Context, args string, opts ...tool.Option) (string, error) {
	type subtitleParams struct {
		VideoID    string `json:"video_id"`
		Action     string `json:"action"`
		TargetLang string `json:"target_lang"`
	}
	params, err := UnmarshalArgs[subtitleParams](subtitleToolName, args)
	if err != nil {
		return "", err
	}
	if err := RequireString(subtitleToolName, "video_id", params.VideoID); err != nil {
		return "", err
	}
	if err := RequireString(subtitleToolName, "action", params.Action); err != nil {
		return "", err
	}
	if params.TargetLang == "" {
		params.TargetLang = "zh-CN"
	}

	// Find subtitle path
	srtContent, err := t.loadSubtitle(ctx, params.VideoID)
	if err != nil {
		return "", err
	}

	switch strings.ToLower(params.Action) {
	case "summarize":
		return t.summarize(ctx, params.VideoID, srtContent)
	case "translate":
		return t.translate(ctx, params.VideoID, srtContent, params.TargetLang)
	default:
		return "", fmt.Errorf("未知动作 %q，请使用 summarize 或 translate", params.Action)
	}
}

// loadSubtitle finds and reads the SRT/VTT for the given video.
func (t *SubtitleTool) loadSubtitle(ctx context.Context, videoID string) (string, error) {
	// 1. Try subtitle_path stored in DB
	var video struct {
		SubtitlePath string `gorm:"column:subtitle_path"`
		VideoPath    string `gorm:"column:video_path"`
	}
	if err := t.db.WithContext(ctx).Table("tb_videos").
		Select("subtitle_path, video_path").
		Where("video_id = ?", videoID).
		First(&video).Error; err != nil {
		return "", fmt.Errorf("未找到视频 %s: %w", videoID, err)
	}

	// Try stored subtitle_path first (must be .srt or .vtt)
	if p := video.SubtitlePath; p != "" && (strings.HasSuffix(p, ".srt") || strings.HasSuffix(p, ".vtt")) {
		if content, err := os.ReadFile(p); err == nil {
			return string(content), nil
		}
	}

	// Guess from download directory: downloads/{video_id}/{video_id}.srt
	guesses := []string{
		filepath.Join(t.downloadDir, videoID, videoID+".srt"),
		filepath.Join(t.downloadDir, videoID, videoID+".vtt"),
	}
	// Also try the video_path directory
	if video.VideoPath != "" {
		dir := filepath.Dir(video.VideoPath)
		guesses = append(guesses,
			filepath.Join(dir, videoID+".srt"),
			filepath.Join(dir, videoID+".vtt"),
		)
	}
	for _, g := range guesses {
		if content, err := os.ReadFile(g); err == nil {
			return string(content), nil
		}
	}
	return "", fmt.Errorf("视频 %s 没有可用的字幕文件，请先完成转录步骤", videoID)
}

// summarize uses LLM to produce a summary of the video content.
func (t *SubtitleTool) summarize(ctx context.Context, videoID, srtContent string) (string, error) {
	// Strip timecodes — only keep text lines
	text := stripSRTTimecodes(srtContent)
	if len([]rune(text)) > 8000 {
		text = string([]rune(text)[:8000]) + "\n...(内容截断)"
	}

	prompt := fmt.Sprintf(`请根据以下视频字幕内容，用中文生成一份结构清晰的摘要，包括：
1. 视频主题（1-2句）
2. 主要内容要点（3-6条）
3. 重要结论或行动建议（如有）

字幕内容：
%s`, text)

	resolvedModel := t.resolveActionModel(ctx, "summarize")
	llmClient, err := t.resolveLLMClient(ctx, resolvedModel)
	if err != nil {
		return "", err
	}

	response, err := llmClient.ChatWithOptions(ctx, []llm.Message{
		{Role: "system", Content: "你是一位专业的视频内容分析师，擅长提炼视频核心要点。"},
		{Role: "user", Content: prompt},
	}, llm.ChatOptions{Model: resolvedModel})
	if err != nil {
		return "", fmt.Errorf("LLM调用失败: %w", err)
	}
	return fmt.Sprintf("视频 %s 内容摘要：\n\n%s", videoID, response), nil
}

// translate uses LLM to translate subtitle content to a target language.
func (t *SubtitleTool) translate(ctx context.Context, videoID, srtContent, targetLang string) (string, error) {
	// Only translate text, keep timecodes
	lines := strings.Split(srtContent, "\n")
	var textLines []int
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || isTimecode(trimmed) || isIndexLine(trimmed) {
			continue
		}
		textLines = append(textLines, i)
	}

	// Collect unique text chunks (max 200 lines to avoid token overflow)
	maxLines := 200
	if len(textLines) > maxLines {
		textLines = textLines[:maxLines]
	}

	texts := make([]string, len(textLines))
	for i, idx := range textLines {
		texts[i] = lines[idx]
	}

	prompt := fmt.Sprintf(`请将以下字幕文本翻译成 %s，保持每行分隔，直接输出翻译结果，不要添加解释或标记：

%s`, targetLang, strings.Join(texts, "\n"))

	resolvedModel := t.resolveActionModel(ctx, "translate")
	llmClient, err := t.resolveLLMClient(ctx, resolvedModel)
	if err != nil {
		return "", err
	}

	response, err := llmClient.ChatWithOptions(ctx, []llm.Message{
		{Role: "system", Content: "你是专业翻译，请准确翻译字幕内容，保留语气和意思。"},
		{Role: "user", Content: prompt},
	}, llm.ChatOptions{Model: resolvedModel})
	if err != nil {
		return "", fmt.Errorf("LLM调用失败: %w", err)
	}

	// Reconstruct SRT with translated lines
	translated := strings.Split(strings.TrimSpace(response), "\n")
	result := make([]string, len(lines))
	copy(result, lines)
	for i, idx := range textLines {
		if i < len(translated) {
			result[idx] = strings.TrimSpace(translated[i])
		}
	}

	t.logger.Info("字幕翻译完成",
		zap.String("video_id", videoID),
		zap.String("target_lang", targetLang),
		zap.String("model", resolvedModel),
		zap.Int("lines_translated", len(textLines)))

	return fmt.Sprintf("视频 %s 字幕已翻译为 %s（前%d行文本）：\n\n%s",
		videoID, targetLang, len(textLines), strings.Join(result, "\n")), nil
}

// ── SRT helpers ───────────────────────────────────────────────────────────────

func stripSRTTimecodes(srt string) string {
	var out []string
	for _, line := range strings.Split(srt, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || isTimecode(t) || isIndexLine(t) {
			continue
		}
		out = append(out, t)
	}
	return strings.Join(out, " ")
}

func isTimecode(s string) bool {
	return strings.Contains(s, "-->")
}

func isIndexLine(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/zolagz/ytb2bili/pkg/llm"
	storemodel "github.com/zolagz/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const rewriteMetadataToolName = "rewrite_metadata"

// RewriteMetadataTool regenerates B站 title / description / tags for an existing video via LLM.
type RewriteMetadataTool struct {
	db           *gorm.DB
	llmClient    *llm.EinoChatClient
	userSettings UserSettingsProvider
	logger       *zap.Logger
	userID       string // 由 AgentHandler 每次请求前通过 SetUserContext 注入
}

// NewRewriteMetadataTool creates a RewriteMetadataTool.
func NewRewriteMetadataTool(db *gorm.DB, llmClient *llm.EinoChatClient, userSettings UserSettingsProvider, logger *zap.Logger) *RewriteMetadataTool {
	return &RewriteMetadataTool{db: db, llmClient: llmClient, userSettings: userSettings, logger: logger}
}

// SetUserContext implements ContextualTool — called by AgentHandler before each Run.
func (t *RewriteMetadataTool) SetUserContext(userID string) { t.userID = userID }

func (t *RewriteMetadataTool) resolveLLMClient(ctx context.Context) (*llm.EinoChatClient, error) {
	if t.llmClient == nil {
		return nil, fmt.Errorf("LLM client unavailable for metadata rewrite")
	}
	return t.llmClient, nil
}

func (t *RewriteMetadataTool) resolveMetadataModel(ctx context.Context) string {
	modelName := llm.DefaultModel
	if t.userSettings == nil || !t.userSettings.IsEnabled() || strings.TrimSpace(t.userID) == "" {
		return modelName
	}
	settings, err := t.userSettings.GetSettings(ctx, t.userID)
	if err != nil {
		t.logger.Warn("加载用户元数据模型失败，回退默认模型", zap.String("user_id", t.userID), zap.Error(err))
		return modelName
	}
	if value := strings.TrimSpace(settings[storemodel.UserSettingKeyMetadataModel]); value != "" {
		return value
	}
	if value := strings.TrimSpace(settings[storemodel.UserSettingKeyPreferredAIModel]); value != "" {
		return value
	}
	return modelName
}

// Info describes the tool to the LLM.
func (t *RewriteMetadataTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "rewrite_metadata",
		Desc: "为已下载视频重新生成B站标题、描述和标签（使用AI），结果自动保存。可附加创作风格提示，如「突出编程教程」「面向初学者」「添加emoji」。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"video_id": {
				Type:     schema.String,
				Desc:     "YouTube视频ID，如 dQw4w9WgXcQ",
				Required: true,
			},
			"hint": {
				Type: schema.String,
				Desc: "创作风格提示（可选），如：突出编程教程风格 / 面向初学者 / 标题加emoji",
			},
		}),
	}, nil
}

// InvokableRun executes the metadata rewrite.
func (t *RewriteMetadataTool) InvokableRun(ctx context.Context, args string, opts ...tool.Option) (string, error) {
	type metadataParams struct {
		VideoID string `json:"video_id"`
		Hint    string `json:"hint"`
	}
	params, err := UnmarshalArgs[metadataParams](rewriteMetadataToolName, args)
	if err != nil {
		return "", err
	}
	if err := RequireString(rewriteMetadataToolName, "video_id", params.VideoID); err != nil {
		return "", err
	}

	// Read video info from DB
	var video struct {
		Title       string `gorm:"column:title"`
		Description string `gorm:"column:description"`
	}
	if err := t.db.WithContext(ctx).Table("tb_videos").
		Select("title, description").
		Where("video_id = ?", params.VideoID).
		First(&video).Error; err != nil {
		return "", fmt.Errorf("未找到视频 %s: %w", params.VideoID, err)
	}

	// Build prompt
	extraReq := ""
	if params.Hint != "" {
		extraReq = "\n额外要求: " + params.Hint
	}
	prompt := fmt.Sprintf(
		`你是一位专业的B站UP主助手，请根据以下YouTube视频信息，生成适合B站平台的标题、描述和标签。

YouTube原标题: %s
YouTube原描述（前500字）: %s
%s
要求：
- 标题：吸引人、不超过80字、符合B站风格
- 描述：200-500字、介绍视频亮点、适当使用emoji
- 标签：8-12个，逗号分隔

请严格按JSON格式返回（不含markdown标记）：
{"title": "...", "description": "...", "tags": ["tag1", "tag2"]}`,
		video.Title,
		truncateRunes(video.Description, 500),
		extraReq,
	)

	llmClient, err := t.resolveLLMClient(ctx)
	if err != nil {
		return "", err
	}
	resolvedModel := t.resolveMetadataModel(ctx)
	t.logger.Info("resolved rewrite metadata model",
		zap.String("user_id", strings.TrimSpace(t.userID)),
		zap.String("video_id", params.VideoID),
		zap.String("model", resolvedModel))

	response, err := llmClient.ChatWithOptions(ctx, []llm.Message{
		{Role: "system", Content: "你是专业的B站内容创作助手，擅长生成吸引人的视频标题和描述。"},
		{Role: "user", Content: prompt},
	}, llm.ChatOptions{
		Model: resolvedModel,
	})
	if err != nil {
		return "", fmt.Errorf("LLM调用失败: %w", err)
	}

	// Parse JSON
	var meta struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
	}
	if err := sonic.UnmarshalString(stripMarkdownFence(response), &meta); err != nil {
		return "", fmt.Errorf("LLM返回格式错误: %w\n原始响应:\n%s", err, response)
	}
	if meta.Title == "" {
		return "", fmt.Errorf("LLM未生成标题")
	}

	// Save to DB
	tags := strings.Join(meta.Tags, ",")
	if err := t.db.WithContext(ctx).Table("tb_videos").
		Where("video_id = ?", params.VideoID).
		Updates(map[string]interface{}{
			"generated_title": meta.Title,
			"generated_desc":  meta.Description,
			"generated_tags":  tags,
			"updated_at":      time.Now(),
		}).Error; err != nil {
		t.logger.Warn("保存元数据失败", zap.String("video_id", params.VideoID), zap.Error(err))
	}

	return fmt.Sprintf("✅ 元数据已更新！\n\n标题: %s\n\n描述: %s\n\n标签: %s", meta.Title, meta.Description, tags), nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func truncateRunes(s string, n int) string {
	runes := []rune(s)
	if len(runes) > n {
		return string(runes[:n]) + "..."
	}
	return s
}

func stripMarkdownFence(s string) string {
	s = strings.TrimSpace(s)
	for _, prefix := range []string{"```json", "```"} {
		if strings.HasPrefix(s, prefix) {
			s = strings.TrimPrefix(s, prefix)
			s = strings.TrimSuffix(s, "```")
			s = strings.TrimSpace(s)
			break
		}
	}
	return s
}

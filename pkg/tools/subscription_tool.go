package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const subscriptionToolName = "manage_subscription"

// tbSubscriptionModel is a minimal model for GORM delete with soft-delete support.
type tbSubscriptionModel struct{}

func (tbSubscriptionModel) TableName() string { return "tb_subscription" }

// SubscriptionTool manages YouTube/Bilibili channel subscriptions via the database.
type SubscriptionTool struct {
	db     *gorm.DB
	logger *zap.Logger
	userID string // 由 AgentHandler 每次请求前通过 SetUserContext 注入
}

// NewSubscriptionTool creates a SubscriptionTool.
func NewSubscriptionTool(db *gorm.DB, logger *zap.Logger) *SubscriptionTool {
	return &SubscriptionTool{db: db, logger: logger}
}

// SetUserContext implements ContextualTool — called by AgentHandler before each Run.
func (t *SubscriptionTool) SetUserContext(userID string) { t.userID = userID }

// Info describes the tool to the LLM.
func (t *SubscriptionTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "manage_subscription",
		Desc: "管理频道订阅。支持三个操作：list(列出所有订阅)、add(添加订阅)、remove(取消订阅)。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"action": {
				Type:     schema.String,
				Desc:     "操作: list(列出订阅)、add(添加订阅)、remove(取消订阅)",
				Required: true,
			},
			"channel_id": {
				Type: schema.String,
				Desc: "频道ID（add/remove时必填，如 UCxxxxxx）",
			},
			"channel_title": {
				Type: schema.String,
				Desc: "频道名称（add时可选，方便记忆）",
			},
			"platform": {
				Type: schema.String,
				Desc: "平台: youtube 或 bilibili，默认 youtube",
			},
			"user_id": {
				Type: schema.String,
				Desc: "用户ID（可选）",
			},
		}),
	}, nil
}

type subParams struct {
	Action       string `json:"action"`
	ChannelID    string `json:"channel_id"`
	ChannelTitle string `json:"channel_title"`
	Platform     string `json:"platform"`
	UserID       string `json:"user_id"`
}

// InvokableRun executes the subscription action.
func (t *SubscriptionTool) InvokableRun(ctx context.Context, args string, opts ...tool.Option) (string, error) {
	params, err := UnmarshalArgs[subParams](subscriptionToolName, args)
	if err != nil {
		return "", err
	}
	if err := RequireString(subscriptionToolName, "action", params.Action); err != nil {
		return "", err
	}
	if params.Platform == "" {
		params.Platform = "youtube"
	}

	// 工具注入的 userID 优先；LLM 传入的 params.UserID 作为后备（兼容旧行为）
	effectiveUID := t.userID
	if effectiveUID == "" {
		effectiveUID = params.UserID
	}

	switch strings.ToLower(params.Action) {
	case "list":
		return t.listSubscriptions(ctx, params.Platform, effectiveUID)
	case "add":
		return t.addSubscription(ctx, params.ChannelID, params.ChannelTitle, params.Platform, effectiveUID)
	case "remove":
		return t.removeSubscription(ctx, params.ChannelID, params.Platform, effectiveUID)
	default:
		return "", ToolError(subscriptionToolName, fmt.Sprintf("unknown action %q, use list, add, or remove", params.Action))
	}
}

func (t *SubscriptionTool) listSubscriptions(ctx context.Context, platform, userID string) (string, error) {
	type row struct {
		ChannelID    string    `gorm:"column:channel_id"    json:"channel_id"`
		ChannelTitle string    `gorm:"column:channel_title" json:"channel_title"`
		Platform     string    `gorm:"column:platform"      json:"platform"`
		Status       string    `gorm:"column:status"        json:"status"`
		SyncedAt     time.Time `gorm:"column:synced_at"     json:"last_synced"`
	}

	q := t.db.WithContext(ctx).Table("tb_subscription").
		Select("channel_id, channel_title, platform, status, synced_at").
		Order("subscribed_at DESC").
		Limit(50)

	if platform != "" && platform != "all" {
		q = q.Where("platform = ?", platform)
	}
	if userID != "" {
		q = q.Where("user_id = ?", userID)
	}

	var rows []row
	if err := q.Find(&rows).Error; err != nil {
		return "", fmt.Errorf("查询失败: %w", err)
	}
	if len(rows) == 0 {
		return "当前没有订阅任何频道", nil
	}

	b, _ := json.MarshalIndent(rows, "", "  ")
	return fmt.Sprintf("共订阅了 %d 个频道:\n%s", len(rows), string(b)), nil
}

func (t *SubscriptionTool) addSubscription(ctx context.Context, channelID, channelTitle, platform, userID string) (string, error) {
	if channelID == "" {
		return "", fmt.Errorf("channel_id 不能为空")
	}

	// Check if already subscribed
	var count int64
	t.db.WithContext(ctx).Table("tb_subscription").
		Where("channel_id = ? AND platform = ?", channelID, platform).
		Count(&count)
	if count > 0 {
		return fmt.Sprintf("频道 %s (%s) 已在订阅列表中", channelTitle, channelID), nil
	}

	sub := map[string]interface{}{
		"user_id":       userID,
		"channel_id":    channelID,
		"channel_title": channelTitle,
		"platform":      platform,
		"status":        "active",
		"subscribed_at": time.Now(),
		"synced_at":     time.Now(),
		"created_at":    time.Now(),
		"updated_at":    time.Now(),
	}

	if err := t.db.WithContext(ctx).Table("tb_subscription").Create(&sub).Error; err != nil {
		return "", fmt.Errorf("添加订阅失败: %w", err)
	}

	t.logger.Info("添加订阅", zap.String("channel_id", channelID), zap.String("platform", platform))
	name := channelTitle
	if name == "" {
		name = channelID
	}
	return fmt.Sprintf("✅ 已订阅频道「%s」(%s)。下次同步时将自动获取新视频。", name, channelID), nil
}

func (t *SubscriptionTool) removeSubscription(ctx context.Context, channelID, platform, userID string) (string, error) {
	if channelID == "" {
		return "", fmt.Errorf("channel_id 不能为空")
	}

	q := t.db.WithContext(ctx).Table("tb_subscription").
		Where("channel_id = ? AND platform = ?", channelID, platform)
	if userID != "" {
		q = q.Where("user_id = ?", userID)
	}

	result := q.Delete(&tbSubscriptionModel{})
	if result.Error != nil {
		return "", fmt.Errorf("取消订阅失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Sprintf("未找到频道 %s 的订阅记录", channelID), nil
	}

	t.logger.Info("取消订阅", zap.String("channel_id", channelID))
	return fmt.Sprintf("✅ 已取消订阅频道 %s", channelID), nil
}

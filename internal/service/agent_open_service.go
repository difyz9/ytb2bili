package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zolagz/ytb2bili/internal/config"
	"github.com/zolagz/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	ErrAgentOpenDisabled = errors.New("agent open api disabled")
	ErrInvalidAPIKey     = errors.New("invalid api key")
	ErrInsufficientScope = errors.New("insufficient scope")
	ErrToolNotFound      = errors.New("tool not found")
	ErrToolNotImplemented = errors.New("tool not implemented")
	ErrJobNotFound       = errors.New("job not found")
)

const (
	AgentOpenModeSync  = "sync"
	AgentOpenModeAsync = "async"
)

type AgentOpenToolDefinition struct {
	Name             string         `json:"name"`
	Title            string         `json:"title"`
	Description      string         `json:"description"`
	Mode             string         `json:"mode"`
	Scopes           []string       `json:"scopes"`
	EstimatedCredits float64        `json:"estimated_credits"`
	InputSchema      map[string]any `json:"input_schema,omitempty"`
	OutputSchema     map[string]any `json:"output_schema,omitempty"`
}

type AgentOpenPrincipal struct {
	ClientID    string
	OwnerID     string
	KeyID       string
	Scopes      map[string]struct{}
	RateLimit   int
	RawScopeCSV string
}

func (p *AgentOpenPrincipal) HasScopes(required ...string) bool {
	if p == nil {
		return false
	}
	for _, scope := range required {
		if _, ok := p.Scopes[scope]; !ok {
			return false
		}
	}
	return true
}

type AgentOpenInvocationContext struct {
	OwnerUserID     string            `json:"owner_user_id"`
	DelegatedUserID string            `json:"delegated_user_id"`
	Tags            map[string]string `json:"tags"`
}

type AgentOpenWebhookConfig struct {
	URL    string   `json:"url"`
	Events []string `json:"events"`
}

type AgentOpenJobRequest struct {
	ToolName string                     `json:"tool_name" binding:"required"`
	Input    map[string]any             `json:"input" binding:"required"`
	Context  AgentOpenInvocationContext `json:"context"`
	Webhook  *AgentOpenWebhookConfig    `json:"webhook"`
}

type AgentOpenService struct {
	cfg    config.AgentOpenAPIConfig
	db     *gorm.DB
	logger *zap.Logger
	tools  []AgentOpenToolDefinition
}

func NewAgentOpenService(cfg *config.AppConfig, db *gorm.DB, logger *zap.Logger) *AgentOpenService {
	return &AgentOpenService{
		cfg:    cfg.AgentOpenAPI,
		db:     db,
		logger: logger,
		tools:  defaultAgentOpenToolCatalog(),
	}
}

func (s *AgentOpenService) Enabled() bool {
	return s != nil && s.cfg.Enabled
}

func (s *AgentOpenService) BaseURL() string {
	if s == nil {
		return ""
	}
	return s.cfg.BaseURL
}

func (s *AgentOpenService) DefaultRateLimit() int {
	if s == nil || s.cfg.DefaultRateLimit <= 0 {
		return 60
	}
	return s.cfg.DefaultRateLimit
}

func (s *AgentOpenService) Capabilities() map[string]any {
	return map[string]any{
		"service":  "ytb2bili-agent-open-api",
		"version":  "v1",
		"base_url": s.BaseURL(),
		"protocols": []string{"rest", "openapi", "mcp"},
		"rate_limit": map[string]any{
			"requests_per_minute": s.DefaultRateLimit(),
			"burst":               10,
		},
		"features": map[string]any{
			"tool_invocation":  true,
			"async_jobs":       true,
			"resource_download": false,
			"agent_run":        false,
		},
	}
}

func (s *AgentOpenService) ListTools(mode string) []AgentOpenToolDefinition {
	if mode == "" || mode == "all" {
		return append([]AgentOpenToolDefinition(nil), s.tools...)
	}
	out := make([]AgentOpenToolDefinition, 0, len(s.tools))
	for _, tool := range s.tools {
		if tool.Mode == mode {
			out = append(out, tool)
		}
	}
	return out
}

func (s *AgentOpenService) GetTool(name string) (AgentOpenToolDefinition, bool) {
	for _, tool := range s.tools {
		if tool.Name == name {
			return tool, true
		}
	}
	return AgentOpenToolDefinition{}, false
}

func (s *AgentOpenService) AuthenticateAPIKey(ctx context.Context, rawKey string) (*AgentOpenPrincipal, error) {
	if !s.Enabled() {
		return nil, ErrAgentOpenDisabled
	}
	if strings.TrimSpace(rawKey) == "" {
		return nil, ErrInvalidAPIKey
	}

	keyHash := hashAgentOpenAPIKey(s.cfg.APIKeyHashSecret, rawKey)
	var apiKey model.AgentAPIKey
	if err := s.db.WithContext(ctx).
		Where("key_hash = ? AND status = ?", keyHash, 1).
		First(&apiKey).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidAPIKey
		}
		return nil, err
	}
	if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
		return nil, ErrInvalidAPIKey
	}

	var client model.AgentClient
	if err := s.db.WithContext(ctx).
		Where("client_id = ? AND status = ?", apiKey.ClientID, 1).
		First(&client).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidAPIKey
		}
		return nil, err
	}

	now := time.Now()
	_ = s.db.WithContext(ctx).
		Model(&model.AgentAPIKey{}).
		Where("id = ?", apiKey.ID).
		Update("last_used_at", &now).Error

	rateLimit := client.DefaultRateLimit
	if rateLimit <= 0 {
		rateLimit = s.DefaultRateLimit()
	}

	return &AgentOpenPrincipal{
		ClientID:    client.ClientID,
		OwnerID:     client.OwnerID,
		KeyID:       apiKey.KeyID,
		Scopes:      parseAgentOpenScopes(apiKey.Scopes),
		RateLimit:   rateLimit,
		RawScopeCSV: apiKey.Scopes,
	}, nil
}

func (s *AgentOpenService) CreateJob(ctx context.Context, principal *AgentOpenPrincipal, req AgentOpenJobRequest, requestID, idempotencyKey string) (*model.AgentJob, error) {
	tool, ok := s.GetTool(req.ToolName)
	if !ok {
		return nil, ErrToolNotFound
	}
	if tool.Mode != AgentOpenModeAsync {
		return nil, fmt.Errorf("tool %s is not async", req.ToolName)
	}
	if idempotencyKey != "" {
		var existing model.AgentJob
		err := s.db.WithContext(ctx).
			Where("client_id = ? AND idempotency_key = ?", principal.ClientID, idempotencyKey).
			First(&existing).Error
		if err == nil {
			return &existing, nil
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	inputJSON, err := json.Marshal(req.Input)
	if err != nil {
		return nil, err
	}

	job := &model.AgentJob{
		JobID:           newAgentOpenJobID(),
		ClientID:        principal.ClientID,
		ToolName:        req.ToolName,
		Status:          "queued",
		Progress:        0,
		Stage:           "queued",
		InputJSON:       string(inputJSON),
		RequestID:       requestID,
		IdempotencyKey:  idempotencyKey,
		OwnerUserID:     fallbackString(req.Context.OwnerUserID, principal.OwnerID),
		DelegatedUserID: req.Context.DelegatedUserID,
	}
	if req.Webhook != nil {
		job.WebhookURL = req.Webhook.URL
		job.WebhookEvents = strings.Join(req.Webhook.Events, ",")
		job.WebhookStatus = "pending"
	}
	if err := s.db.WithContext(ctx).Create(job).Error; err != nil {
		return nil, err
	}
	return job, nil
}

func (s *AgentOpenService) InvokeTool(ctx context.Context, principal *AgentOpenPrincipal, toolName string, input map[string]any, invCtx AgentOpenInvocationContext) (map[string]any, float64, error) {
	tool, ok := s.GetTool(toolName)
	if !ok {
		return nil, 0, ErrToolNotFound
	}
	if tool.Mode != AgentOpenModeSync {
		return nil, 0, ErrToolNotImplemented
	}

	switch toolName {
	case "query_videos":
		return s.invokeQueryVideos(ctx, principal, input, invCtx)
	case "manage_subscription":
		return s.invokeManageSubscription(ctx, principal, input, invCtx)
	default:
		return nil, 0, ErrToolNotImplemented
	}
}

func (s *AgentOpenService) GetJob(ctx context.Context, principal *AgentOpenPrincipal, jobID string) (*model.AgentJob, error) {
	var job model.AgentJob
	if err := s.db.WithContext(ctx).
		Where("job_id = ? AND client_id = ?", jobID, principal.ClientID).
		First(&job).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrJobNotFound
		}
		return nil, err
	}
	return &job, nil
}

func defaultAgentOpenToolCatalog() []AgentOpenToolDefinition {
	return []AgentOpenToolDefinition{
		{
			Name:             "query_videos",
			Title:            "Query Videos",
			Description:      "Query the user's video library with filters",
			Mode:             AgentOpenModeSync,
			Scopes:           []string{"tools.invoke"},
			EstimatedCredits: 0,
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"status":   map[string]any{"type": "string"},
					"platform": map[string]any{"type": "string"},
					"limit":    map[string]any{"type": "integer", "minimum": 1, "maximum": 100},
				},
			},
			OutputSchema: map[string]any{"type": "object"},
		},
		{
			Name:             "manage_subscription",
			Title:            "Manage Subscription",
			Description:      "List, add, or remove subscriptions",
			Mode:             AgentOpenModeSync,
			Scopes:           []string{"tools.invoke"},
			EstimatedCredits: 0,
			InputSchema:      map[string]any{"type": "object"},
			OutputSchema:     map[string]any{"type": "object"},
		},
		{
			Name:             "rewrite_metadata",
			Title:            "Rewrite Metadata",
			Description:      "Generate title, description, and tags for a video",
			Mode:             AgentOpenModeSync,
			Scopes:           []string{"tools.invoke"},
			EstimatedCredits: 1,
			InputSchema:      map[string]any{"type": "object"},
			OutputSchema:     map[string]any{"type": "object"},
		},
		{
			Name:             "subtitle_action",
			Title:            "Subtitle Action",
			Description:      "Summarize or translate subtitles",
			Mode:             AgentOpenModeAsync,
			Scopes:           []string{"jobs.create"},
			EstimatedCredits: 2,
			InputSchema:      map[string]any{"type": "object"},
			OutputSchema:     map[string]any{"type": "object"},
		},
		{
			Name:             "submit_pipeline",
			Title:            "Submit Pipeline",
			Description:      "Submit a full video processing pipeline job",
			Mode:             AgentOpenModeAsync,
			Scopes:           []string{"jobs.create"},
			EstimatedCredits: 2,
			InputSchema:      map[string]any{"type": "object"},
			OutputSchema:     map[string]any{"type": "object"},
		},
		{
			Name:             "download_video",
			Title:            "Download Video",
			Description:      "Download a remote video and expose it as a resource",
			Mode:             AgentOpenModeAsync,
			Scopes:           []string{"jobs.create"},
			EstimatedCredits: 1,
			InputSchema:      map[string]any{"type": "object"},
			OutputSchema:     map[string]any{"type": "object"},
		},
	}
}


func (s *AgentOpenService) invokeQueryVideos(ctx context.Context, principal *AgentOpenPrincipal, input map[string]any, invCtx AgentOpenInvocationContext) (map[string]any, float64, error) {
	ownerUserID := fallbackString(invCtx.OwnerUserID, principal.OwnerID)
	if ownerUserID == "" {
		return nil, 0, fmt.Errorf("owner user id is required")
	}

	query := s.db.WithContext(ctx).Model(&model.Video{}).Where("user_id = ?", ownerUserID)
	if status := asString(input["status"]); status != "" {
		query = query.Where("status = ?", status)
	}
	if platform := asString(input["platform"]); platform != "" {
		query = query.Where("platform = ?", platform)
	}

	limit := asInt(input["limit"], 20)
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	var videos []model.Video
	if err := query.Order("updated_at DESC").Limit(limit).Find(&videos).Error; err != nil {
		return nil, 0, err
	}

	items := make([]map[string]any, 0, len(videos))
	for _, video := range videos {
		items = append(items, map[string]any{
			"video_id":      video.VideoID,
			"title":         video.Title,
			"status":        video.Status,
			"platform":      video.Platform,
			"url":           video.URL,
			"thumbnail":     video.Thumbnail,
			"bili_bvid":     video.BiliBVID,
			"updated_at":    video.UpdatedAt,
			"published_at":  video.PublishedAt,
			"generated_title": video.GeneratedTitle,
		})
	}

	return map[string]any{
		"videos": items,
		"count":  len(items),
	}, 0, nil
}

func (s *AgentOpenService) invokeManageSubscription(ctx context.Context, principal *AgentOpenPrincipal, input map[string]any, invCtx AgentOpenInvocationContext) (map[string]any, float64, error) {
	ownerUserID := fallbackString(invCtx.OwnerUserID, principal.OwnerID)
	if ownerUserID == "" {
		return nil, 0, fmt.Errorf("owner user id is required")
	}

	action := strings.ToLower(asString(input["action"]))
	switch action {
	case "", "list":
		var subs []model.TbSubscription
		query := s.db.WithContext(ctx).Where("user_id = ?", ownerUserID)
		if platform := asString(input["platform"]); platform != "" {
			query = query.Where("platform = ?", platform)
		}
		if err := query.Order("updated_at DESC").Find(&subs).Error; err != nil {
			return nil, 0, err
		}
		return map[string]any{"subscriptions": subs, "count": len(subs)}, 0, nil
	case "add":
		channelID := asString(input["channel_id"])
		platform := asString(input["platform"])
		if channelID == "" || platform == "" {
			return nil, 0, fmt.Errorf("channel_id and platform are required")
		}
		now := time.Now()
		sub := model.TbSubscription{
			UserID:              ownerUserID,
			ChannelID:           channelID,
			Platform:            platform,
			ChannelTitle:        asString(input["channel_title"]),
			ChannelDescription:  asString(input["channel_description"]),
			ChannelThumbnailURL: asString(input["channel_thumbnail_url"]),
			ChannelCustomURL:    asString(input["channel_custom_url"]),
			SubscribedAt:        now,
			SyncedAt:            now,
			Status:              "active",
		}
		if err := s.db.WithContext(ctx).
			Where("user_id = ? AND channel_id = ?", ownerUserID, channelID).
			Assign(sub).
			FirstOrCreate(&sub).Error; err != nil {
			return nil, 0, err
		}
		return map[string]any{"subscription": sub, "action": "add"}, 0, nil
	case "remove":
		channelID := asString(input["channel_id"])
		if channelID == "" {
			return nil, 0, fmt.Errorf("channel_id is required")
		}
		result := s.db.WithContext(ctx).Where("user_id = ? AND channel_id = ?", ownerUserID, channelID).Delete(&model.TbSubscription{})
		if result.Error != nil {
			return nil, 0, result.Error
		}
		return map[string]any{"removed": result.RowsAffected > 0, "channel_id": channelID, "action": "remove"}, 0, nil
	default:
		return nil, 0, fmt.Errorf("unsupported action: %s", action)
	}
}

func parseAgentOpenScopes(raw string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, item := range strings.Split(raw, ",") {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			set[trimmed] = struct{}{}
		}
	}
	return set
}

func hashAgentOpenAPIKey(secret, raw string) string {
	sum := sha256.Sum256([]byte(secret + ":" + raw))
	return hex.EncodeToString(sum[:])
}

func newAgentOpenJobID() string {
	return fmt.Sprintf("job_%d", time.Now().UnixNano())
}

func fallbackString(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func asString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}

func asInt(value any, fallback int) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	default:
		return fallback
	}
}
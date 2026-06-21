package model

import "time"

// AgentClient 表示一个可调用开放平台的第三方 agent 应用。
type AgentClient struct {
	BaseModel
	ClientID         string `gorm:"uniqueIndex;size:64;not null" json:"client_id"`
	Name             string `gorm:"size:100;not null" json:"name"`
	Description      string `gorm:"size:255" json:"description"`
	OwnerID          string `gorm:"size:128;index;not null" json:"owner_id"`
	Status           int    `gorm:"default:1;index" json:"status"`
	DefaultRateLimit int    `gorm:"default:60" json:"default_rate_limit"`
	AllowedOrigins   string `gorm:"type:text" json:"allowed_origins"`
	WebhookAllowlist string `gorm:"type:text" json:"webhook_allowlist"`
}

func (AgentClient) TableName() string {
	return "tb_agent_clients"
}

// AgentAPIKey 表示一个外部 agent 使用的 API key 记录。
type AgentAPIKey struct {
	BaseModel
	KeyID      string     `gorm:"uniqueIndex;size:64;not null" json:"key_id"`
	ClientID   string     `gorm:"size:64;index;not null" json:"client_id"`
	KeyPrefix  string     `gorm:"size:16;index" json:"key_prefix"`
	KeyHash    string     `gorm:"uniqueIndex;size:128;not null" json:"-"`
	Scopes     string     `gorm:"type:text" json:"scopes"`
	Status     int        `gorm:"default:1;index" json:"status"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

func (AgentAPIKey) TableName() string {
	return "tb_agent_api_keys"
}

// AgentRequestLog 记录开放平台的请求审计信息。
type AgentRequestLog struct {
	ID             int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ClientID       string    `gorm:"size:64;index;not null" json:"client_id"`
	KeyID          string    `gorm:"size:64;index" json:"key_id"`
	RequestID      string    `gorm:"size:128;index" json:"request_id"`
	Endpoint       string    `gorm:"size:255;index" json:"endpoint"`
	Method         string    `gorm:"size:16" json:"method"`
	ToolName       string    `gorm:"size:64;index" json:"tool_name"`
	HTTPStatus     int       `json:"http_status"`
	ErrorCode      string    `gorm:"size:64" json:"error_code"`
	DurationMS     int64     `json:"duration_ms"`
	CreditsSpent   float64   `json:"credits_spent"`
	OwnerUserID    string    `gorm:"size:128;index" json:"owner_user_id"`
	DelegatedUserID string   `gorm:"size:128;index" json:"delegated_user_id"`
	CreatedAt      time.Time `gorm:"index" json:"created_at"`
}

func (AgentRequestLog) TableName() string {
	return "tb_agent_request_logs"
}

// AgentJob 记录异步能力调用的作业状态。
type AgentJob struct {
	BaseModel
	JobID            string `gorm:"uniqueIndex;size:64;not null" json:"job_id"`
	ClientID         string `gorm:"size:64;index;not null" json:"client_id"`
	ToolName         string `gorm:"size:64;index;not null" json:"tool_name"`
	Status           string `gorm:"size:32;index;not null" json:"status"`
	Progress         int    `gorm:"default:0" json:"progress"`
	Stage            string `gorm:"size:64" json:"stage"`
	InputJSON        string `gorm:"type:mediumtext" json:"-"`
	ResultJSON       string `gorm:"type:mediumtext" json:"-"`
	ErrorCode        string `gorm:"size:64" json:"error_code"`
	ErrorMessage     string `gorm:"type:text" json:"error_message"`
	RequestID        string `gorm:"size:128;index" json:"request_id"`
	IdempotencyKey   string `gorm:"size:128;index" json:"idempotency_key"`
	OwnerUserID      string `gorm:"size:128;index" json:"owner_user_id"`
	DelegatedUserID  string `gorm:"size:128;index" json:"delegated_user_id"`
	WebhookURL       string `gorm:"size:500" json:"webhook_url"`
	WebhookEvents    string `gorm:"type:text" json:"webhook_events"`
	WebhookStatus    string `gorm:"size:32" json:"webhook_status"`
	CreditsSpent     float64 `json:"credits_spent"`
}

func (AgentJob) TableName() string {
	return "tb_agent_jobs"
}
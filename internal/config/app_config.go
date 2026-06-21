package config

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	agent "github.com/zolagz/ytb2bili/pkg/agent"
	"github.com/zolagz/ytb2bili/pkg/llm"
)

// ── ProviderConfig ───────────────────────────────────────────────────────────
// ProviderConfig describes a single LLM provider connection for one use case.
// Defined here for TOML serialization; consumers convert to llm.ProviderConfig.
type ProviderConfig struct {
	Provider    string   `toml:"provider"`
	Model       string   `toml:"model"`
	Models      []string `toml:"models,omitempty"`
	BaseURL     string   `toml:"base_url"`
	APIKey      string   `toml:"api_key"`
	Temperature *float64 `toml:"temperature,omitempty"`
	MaxTokens   *int     `toml:"max_tokens,omitempty"`
	Timeout     *int     `toml:"timeout,omitempty"`
}

// ToLLMConfig converts to the llm package type.
func (p *ProviderConfig) ToLLMConfig() *llm.ProviderConfig {
	if p == nil {
		return nil
	}
	return &llm.ProviderConfig{
		Provider:    p.Provider,
		Model:       p.Model,
		BaseURL:     p.BaseURL,
		APIKey:      p.APIKey,
		Temperature: p.Temperature,
		MaxTokens:   p.MaxTokens,
		Timeout:     p.Timeout,
	}
}

// ── AppConfig ────────────────────────────────────────────────────────────────

// AppConfig is the runtime configuration for the app.
type AppConfig struct {
	Debug bool `toml:"debug"`

	Server   ServerConfig   `toml:"server"`
	Database DatabaseConfig `toml:"database"`
	Auth     AuthConfig     `toml:"auth"`

	// ── LLM providers (new — independent per use case) ──────────────
	Translation *ProviderConfig `toml:"translation,omitempty"` // subtitle translation provider
	Chat        *ProviderConfig `toml:"chat,omitempty"`        // chat / text generation provider

	// ── Legacy LLM config (backward-compatible fallback) ───────────
	LLM      LLMConfig      `toml:"llm"`
	Deepseek DeepseekConfig `toml:"deepseek"` // Deepseek LLM config (legacy)

	// ── Other sections ─────────────────────────────────────────────
	TTS       TTSConfig       `toml:"tts"`
	AzureTTS  AzureTTSConfig  `toml:"azure_tts"`
	TikHub    TikHubConfig    `toml:"tikhub"`
	Feishu    FeishuConfig    `toml:"feishu"`
	APIAuth   AppAuthConfig   `toml:"api_auth"`
	Analytics AnalyticsConfig `toml:"analytics"`
	Updater   UpdaterConfig   `toml:"updater"`
	License   LicenseConfig   `toml:"license"`
	Agent     *agent.Config   `toml:"agent"`

	Workflow     WorkflowConfig     `toml:"workflow"`
	AgentOpenAPI AgentOpenAPIConfig `toml:"agent_open_api"`
}

// LLMConfig holds the user-configurable LLM provider settings.
type LLMConfig struct {
	Provider    string  `toml:"provider"`    // openai, deepseek, anthropic, ollama, qwen, custom
	BaseURL     string  `toml:"base_url"`    // OpenAI-compatible API endpoint
	APIKey      string  `toml:"api_key"`     // User's API key
	Model       string  `toml:"model"`       // Default model name (optional; falls back to Models[0])
	Models      []string `toml:"models,omitempty"` // Selectable model list for frontend
	Temperature float64 `toml:"temperature"` // Default temperature (0.0-2.0)
	MaxTokens   int     `toml:"max_tokens"`  // Default max tokens per request
	Timeout     int     `toml:"timeout"`     // Timeout in seconds (default 120)
}

// DeepseekConfig Deepseek LLM 配置
type DeepseekConfig struct {
	APIKey string `toml:"api_key"` // Deepseek API 密钥
}

type LocalAuthUser struct {
	ID          string `toml:"id"`
	Username    string `toml:"username"`
	Password    string `toml:"password"`
	DisplayName string `toml:"display_name"`
	Email       string `toml:"email"`
	Role        string `toml:"role"`
	Avatar      string `toml:"avatar"`
}

type AuthConfig struct {
	JWTSecret       string          `toml:"jwt_secret"`
	AccessTokenTTL  int             `toml:"access_token_ttl"`
	RefreshTokenTTL int             `toml:"refresh_token_ttl"`
	Users           []LocalAuthUser `toml:"users"`
}

// AgentOpenAPIConfig 对外开放给第三方 agent 的 API 配置。
type AgentOpenAPIConfig struct {
	Enabled              bool     `toml:"enabled"`
	BaseURL              string   `toml:"base_url"`
	APIKeyHashSecret     string   `toml:"api_key_hash_secret"`
	DefaultRateLimit     int      `toml:"default_rate_limit"`
	WebhookSigningSecret string   `toml:"webhook_signing_secret"`
	AllowedOrigins       []string `toml:"allowed_origins"`
}

// FeishuConfig 飞书 Bot 配置
type FeishuConfig struct {
	Enabled           bool   `toml:"enabled"`
	AppID             string `toml:"app_id"`
	AppSecret         string `toml:"app_secret"`
	VerificationToken string `toml:"verification_token"`
	EncryptKey        string `toml:"encrypt_key"`
}

type ServerConfig struct {
	Host       string `toml:"host"`
	Port       int    `toml:"port"`
	StaticDir  string `toml:"static_dir"`
	StaticPath string `toml:"static_path"`
}

type DatabaseConfig struct {
	Type        string `toml:"type"`
	Host        string `toml:"host"`
	Port        int    `toml:"port"`
	User        string `toml:"user"`
	Password    string `toml:"password"`
	DBName      string `toml:"dbname"`
	SSLMode     string `toml:"sslmode"`
	Timezone    string `toml:"timezone"`
	Path        string `toml:"path"`
	TablePrefix string `toml:"table_prefix"`
	AutoMigrate bool   `toml:"auto_migrate"`
}

type TikHubConfig struct {
	APIKey  string `toml:"api_key"`
	BaseURL string `toml:"base_url"`
}

type AzureTTSConfig struct {
	SubscriptionKey string `toml:"subscription_key"`
	Region          string `toml:"region"`
}

type AppAuthConfig struct {
	BaseURL           string `toml:"base_url"`
	AppID             string `toml:"app_id"`
	AppSecret         string `toml:"app_secret"`
	Timeout           int    `toml:"timeout"`
	Enabled           bool   `toml:"enabled"`
	CookiesDecryptKey string `toml:"cookies_decrypt_key"`
}

type AnalyticsConfig struct {
	Enabled     bool   `toml:"enabled"`
	ServerURL   string `toml:"server_url"`
	ProductName string `toml:"product_name"`
	Debug       bool   `toml:"debug"`
}

type UpdaterConfig struct {
	Enabled             bool   `toml:"enabled"`
	AutoUpdate          bool   `toml:"auto_update"`
	RestartOnSuccess    bool   `toml:"restart_on_success"`
	RestartDelaySeconds int    `toml:"restart_delay_seconds"`
	CheckInterval       int    `toml:"check_interval"`
	UpdateURL           string `toml:"update_url"`
	GitHubAPIToken      string `toml:"github_api_token"`
	CurrentVersion      string `toml:"current_version"`
}

// LicenseConfig 激活码验证配置
type LicenseConfig struct {
	VerifyURL string `toml:"verify_url"` // worker 验证端点，如 https://open.licensestore.org
	AdminKey  string `toml:"admin_key"`  // worker 管理 API 密钥
}

// WorkflowConfig 视频处理工作流配置
type WorkflowConfig struct {
	DownloadDir       string `toml:"download_dir"`
	CookiesDir        string `toml:"cookies_dir"`
	CredentialsDir    string `toml:"credentials_dir"`
	YtDlpPath         string `toml:"ytdlp_path"`
	FFmpegPath        string `toml:"ffmpeg_path"`
	WatermarkFontFile string `toml:"watermark_font_file"`
	CookiesFile       string `toml:"cookies_file"`
	ProxyURL          string `toml:"proxy_url"`
	SpeechKey         string `toml:"speech_key"`
	SpeechRegion      string `toml:"speech_region"`

	// LLM翻译配置
	LLMTranslationEnabled     bool   `toml:"llm_translation_enabled"`
	LLMTranslationBatchSize   int    `toml:"llm_translation_batch_size"`
	LLMTranslationMaxWorkers  int    `toml:"llm_translation_max_workers"`
	LLMTranslationContextSize int    `toml:"llm_translation_context_size"`
	LLMTranslationSourceLang  string `toml:"llm_translation_source_lang"`
	LLMTranslationTargetLang  string `toml:"llm_translation_target_lang"`

	// TTS配置
	TTSEnabled bool `toml:"tts_enabled"` // 已弃用，仅为兼容保留

	// 并发控制
	MaxConcurrent int `toml:"max_concurrent"`
}

// GetDSN returns driver DSN for the configured DB type.
func (c DatabaseConfig) GetDSN() string {
	switch c.Type {
	case "postgres", "postgresql":
		sslmode := c.SSLMode
		if sslmode == "" {
			sslmode = "disable"
		}
		tz := c.Timezone
		if tz == "" {
			tz = "UTC"
		}
		return fmt.Sprintf(
			"host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=%s",
			c.Host, c.User, c.Password, c.DBName, c.Port, sslmode, tz,
		)
	case "mysql":
		tz := c.Timezone
		if tz == "" {
			tz = "UTC"
		}
		tz = url.QueryEscape(tz)
		return fmt.Sprintf(
			"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=%s",
			c.User, c.Password, c.Host, c.Port, c.DBName, tz,
		)
	default:
		return ""
	}
}

// ── Provider resolution (new config sections with fallback to legacy) ────────

// ResolveTranslationProvider returns the effective translation provider config.
// Order: [translation] > legacy [llm]
func (cfg *AppConfig) ResolveTranslationProvider() *ProviderConfig {
	if cfg.Translation != nil && cfg.Translation.APIKey != "" {
		return cfg.Translation
	}
	if cfg.Translation != nil && cfg.Translation.Provider == llm.ProviderOllama && cfg.Translation.BaseURL != "" {
		return cfg.Translation
	}
	return cfg.legacyLLMProviderConfig()
}

// ResolveChatProvider returns the effective chat provider config.
// Order: [chat] > legacy [llm]
func (cfg *AppConfig) ResolveChatProvider() *ProviderConfig {
	if cfg.Chat != nil && cfg.Chat.APIKey != "" {
		return cfg.Chat
	}
	if cfg.Chat != nil && cfg.Chat.Provider == llm.ProviderOllama && cfg.Chat.BaseURL != "" {
		return cfg.Chat
	}
	return cfg.legacyLLMProviderConfig()
}

// ResolveAgentProvider returns the effective agent provider config.
// Order: [agent].llm > [chat] > legacy [llm]
func (cfg *AppConfig) ResolveAgentProvider() *ProviderConfig {
	if cfg.Agent != nil {
		trimmedKey := strings.TrimSpace(cfg.Agent.LLM.APIKey)
		trimmedURL := strings.TrimSpace(cfg.Agent.LLM.BaseURL)
		if cfg.Agent.LLM.Provider != "" || trimmedKey != "" || trimmedURL != "" {
			chatProvider := cfg.ResolveChatProvider()
			chatModels := []string(nil)
			fallbackModel := llm.DefaultModel
			if chatProvider != nil {
				chatModels = append(chatModels, normalizedModelList(chatProvider.Models)...)
				if chatProvider.Model != "" {
					fallbackModel = strings.TrimSpace(chatProvider.Model)
				}
			}

			agentModel := resolvePrimaryModel(cfg.Agent.LLM.Model, chatModels, fallbackModel)
			return &ProviderConfig{
				Provider: cfg.Agent.LLM.Provider,
				Model:    agentModel,
				Models:   chatModels,
				BaseURL:  trimmedURL,
				APIKey:   trimmedKey,
			}
		}
	}
	return cfg.ResolveChatProvider()
}

// IsLLMEnabled returns true when at least one LLM provider is configured.
func (cfg *AppConfig) IsLLMEnabled() bool {
	if cfg == nil {
		return false
	}
	return cfg.Translation != nil || cfg.Chat != nil || strings.TrimSpace(cfg.LLM.APIKey) != ""
}

// legacyLLMProviderConfig builds a ProviderConfig from the legacy [llm] section.
func (cfg *AppConfig) legacyLLMProviderConfig() *ProviderConfig {
	models := normalizedModelList(cfg.LLM.Models)
	model := resolvePrimaryModel(cfg.LLM.Model, models, llm.DefaultModel)

	return &ProviderConfig{
		Provider:    cfg.LLM.Provider,
		Model:       model,
		Models:      append([]string(nil), models...),
		BaseURL:     cfg.LLM.BaseURL,
		APIKey:      cfg.LLM.APIKey,
		Temperature: float64Ptr(cfg.LLM.Temperature),
		MaxTokens:   intPtr(cfg.LLM.MaxTokens),
		Timeout:     intPtr(cfg.LLM.Timeout),
	}
}

func float64Ptr(v float64) *float64 {
	return &v
}

func intPtr(v int) *int {
	return &v
}

// ── Backward-compatible helpers ──────────────────────────────────────────────

// LLMBaseURL returns the configured LLM base URL or the default.
func (cfg *AppConfig) LLMBaseURL() string {
	if cfg == nil || strings.TrimSpace(cfg.LLM.BaseURL) == "" {
		return llm.DefaultBaseURL
	}
	return strings.TrimRight(strings.TrimSpace(cfg.LLM.BaseURL), "/")
}

// LLMAPIKey returns the configured LLM API key.
func (cfg *AppConfig) LLMAPIKey() string {
	if cfg == nil {
		return ""
	}
	return strings.TrimSpace(cfg.LLM.APIKey)
}

// LLMModel returns the configured LLM model or the default.
func (cfg *AppConfig) LLMModel() string {
	if cfg == nil {
		return llm.DefaultModel
	}

	return resolvePrimaryModel(cfg.LLM.Model, cfg.LLM.Models, llm.DefaultModel)
}

func resolvePrimaryModel(model string, models []string, fallback string) string {
	trimmedModel := strings.TrimSpace(model)
	if trimmedModel != "" {
		return trimmedModel
	}

	for _, candidate := range models {
		trimmed := strings.TrimSpace(candidate)
		if trimmed != "" {
			return trimmed
		}
	}

	return fallback
}

func normalizedModelList(raw []string) []string {
	result := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))

	for _, modelID := range raw {
		trimmed := strings.TrimSpace(modelID)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}

	return result
}

// GetTTSConfig returns the TTS configuration with defaults applied.
func (cfg *AppConfig) GetTTSConfig() TTSConfig {
	if cfg == nil {
		return TTSConfig{}
	}

	ttsCfg := TTSConfig{}
	if hasExplicitTTSConfig(cfg.TTS) {
		overlayTTSConfig(&ttsCfg, cfg.TTS)
	}

	return ttsCfg
}

// ── Config loading ───────────────────────────────────────────────────────────

// LoadAppConfig loads AppConfig from config.toml file.
func LoadAppConfig() (*AppConfig, error) {
	cfg := getDefaultConfig()

	configFile := "config.toml"
	if _, err := os.Stat(configFile); err == nil {
		if _, err := toml.DecodeFile(configFile, cfg); err != nil {
			return nil, fmt.Errorf("failed to decode config.toml: %w", err)
		}
		log.Printf("📝 已加载配置: %s", configFile)
	} else {
		log.Println("📝 config.toml 不存在，使用默认配置")
		log.Println("💡 提示: 复制 config.toml.example 为 config.toml")
	}

	// Set default ports for database if not specified
	if cfg.Database.Port == 0 {
		switch cfg.Database.Type {
		case "postgres", "postgresql":
			cfg.Database.Port = 5432
		case "mysql":
			cfg.Database.Port = 3306
		}
	}

	// Agent config defaults
	if cfg.Agent == nil {
		cfg.Agent = agent.DefaultConfig()
	} else {
		cfg.Agent.Normalize()
	}

	if err := validateAuthConfig(cfg); err != nil {
		return nil, err
	}

	propagateLLMToAgent(cfg)

	return cfg, nil
}

func validateAuthConfig(cfg *AppConfig) error {
	if cfg == nil {
		return fmt.Errorf("empty config")
	}

	localAuthEnabled := len(cfg.Auth.Users) > 0
	if !localAuthEnabled {
		return nil
	}

	if strings.TrimSpace(cfg.Auth.JWTSecret) == "" {
		if fallback := strings.TrimSpace(cfg.APIAuth.AppSecret); fallback != "" {
			cfg.Auth.JWTSecret = fallback
			log.Printf("⚠️ [auth] jwt_secret 未配置，已回退使用 api_auth.app_secret；建议显式配置 auth.jwt_secret")
		} else {
			cfg.Auth.JWTSecret = "ytb2bili-local-jwt-secret-change-me"
			log.Printf("⚠️ [auth] jwt_secret 未配置，已使用内置默认值；建议尽快在配置文件中设置随机强密钥")
		}
	}

	if cfg.Auth.AccessTokenTTL <= 0 {
		cfg.Auth.AccessTokenTTL = 7200
		log.Printf("⚠️ [auth] access_token_ttl 未配置或无效，已使用默认值 7200 秒")
	}

	if cfg.Auth.RefreshTokenTTL <= 0 {
		cfg.Auth.RefreshTokenTTL = 604800
		log.Printf("⚠️ [auth] refresh_token_ttl 未配置或无效，已使用默认值 604800 秒")
	}

	return nil
}

// AgenticConfig exposes the embedded agent.Config for fx.
func AgenticConfig(cfg *AppConfig) *agent.Config {
	return cfg.Agent
}

// propagateLLMToAgent ensures the Agent config can fall back to [llm] or [chat].
func propagateLLMToAgent(cfg *AppConfig) {
	if cfg == nil || cfg.Agent == nil {
		return
	}

	// If Agent has its own LLM credentials already, keep them.
	if strings.TrimSpace(cfg.Agent.LLM.APIKey) != "" && strings.TrimSpace(cfg.Agent.LLM.BaseURL) != "" {
		return
	}

	// Try [chat] first, then legacy [llm].
	chatCfg := cfg.ResolveChatProvider()
	if chatCfg != nil {
		if strings.TrimSpace(cfg.Agent.LLM.APIKey) == "" {
			cfg.Agent.LLM.APIKey = chatCfg.APIKey
		}
		// Overwrite BaseURL if it is empty or still the built-in default.
		// Without this, agent.DefaultConfig() or Normalize() would pin the URL
		// to DefaultBaseURL (https://api.openai.com) even when [llm] uses DeepSeek.
		if strings.TrimSpace(cfg.Agent.LLM.BaseURL) == "" || cfg.Agent.LLM.BaseURL == llm.DefaultBaseURL {
			cfg.Agent.LLM.BaseURL = chatCfg.BaseURL
		}
		if strings.TrimSpace(cfg.Agent.LLM.Model) == "" || cfg.Agent.LLM.Model == llm.DefaultModel {
			cfg.Agent.LLM.Model = chatCfg.Model
		}
		if strings.TrimSpace(cfg.Agent.LLM.Provider) == "" {
			cfg.Agent.LLM.Provider = chatCfg.Provider
		}
	}
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func hasExplicitTTSConfig(cfg TTSConfig) bool {
	return strings.TrimSpace(cfg.Provider) != "" ||
		strings.TrimSpace(cfg.Voice) != "" ||
		strings.TrimSpace(cfg.Locale) != "" ||
		strings.TrimSpace(cfg.Format) != "" ||
		cfg.TimeoutSeconds != 0 ||
		cfg.Rate != 0 ||
		cfg.Volume != 0 ||
		cfg.Pitch != 0
}

func overlayTTSConfig(target *TTSConfig, source TTSConfig) {
	if target == nil {
		return
	}
	if strings.TrimSpace(source.Provider) != "" {
		target.Provider = source.Provider
	}
	if strings.TrimSpace(source.Voice) != "" {
		target.Voice = source.Voice
	}
	if strings.TrimSpace(source.Locale) != "" {
		target.Locale = source.Locale
	}
	if strings.TrimSpace(source.Format) != "" {
		target.Format = source.Format
	}
	if source.TimeoutSeconds != 0 {
		target.TimeoutSeconds = source.TimeoutSeconds
	}
	if source.Rate != 0 {
		target.Rate = source.Rate
	}
	if source.Volume != 0 {
		target.Volume = source.Volume
	}
	if source.Pitch != 0 {
		target.Pitch = source.Pitch
	}
}

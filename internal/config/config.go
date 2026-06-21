package config

import (
	"bytes"
	"os"

	"github.com/BurntSushi/toml"
	agent "github.com/zolagz/ytb2bili/pkg/agent"
)

const defaultConfigFile = "config.toml"

func resolveConfigPath(path string) string {
	if path != "" {
		return path
	}
	if envPath := os.Getenv("CONFIG_FILE"); envPath != "" {
		return envPath
	}
	return defaultConfigFile
}

func getDefaultConfig() *AppConfig {
	return &AppConfig{
		Debug: false,
		Server: ServerConfig{
			Host:       "0.0.0.0",
			Port:       8096,
			StaticDir:  "./downloads",
			StaticPath: "/static",
		},
		Database: DatabaseConfig{
			Type:        "sqlite",
			Path:        "ytb2bili.db",
			Timezone:    "UTC",
			AutoMigrate: true,
		},
		Auth: AuthConfig{
			JWTSecret:       "",
			AccessTokenTTL:  7200,
			RefreshTokenTTL: 604800,
			Users:           []LocalAuthUser{},
		},
		TTS: TTSConfig{
			Provider:       "auto",
			Voice:          "zh-CN-XiaoxiaoNeural",
			Locale:         "zh-CN",
			Format:         "audio-24khz-96kbitrate-mono-mp3",
			TimeoutSeconds: 180,
			Rate:           1.2,
			Volume:         100,
			Pitch:          0,
		},
		TikHub: TikHubConfig{
			BaseURL: "https://api.tikhub.io",
		},
		Feishu: FeishuConfig{
			Enabled: false,
		},
		APIAuth: AppAuthConfig{
			Enabled:           true,
			AppID:             "ytb2bili_extension",
			AppSecret:         "ytb2bili_secret_2026",
			CookiesDecryptKey: "59e7052041ce4bd6aff82f6a0bca9cde",
		},
		Analytics: AnalyticsConfig{
			Enabled:     true,
			ServerURL:   "https://go-analysis-proxy.vercel.app/proxy",
			ProductName: "ytb2bili",
			Debug:       false,
		},
		Updater: UpdaterConfig{
			Enabled:             true,
			AutoUpdate:          false,
			RestartOnSuccess:    true,
			RestartDelaySeconds: 5,
			CheckInterval:       24,
		},
		License: LicenseConfig{
			VerifyURL: "https://open.licensestore.org",
			AdminKey:  "",
		},
		Workflow: WorkflowConfig{
			DownloadDir:               "./downloads",
			CookiesDir:                "/tmp/cookies",
			CredentialsDir:            "./data/credentials",
			YtDlpPath:                 "/usr/local/bin/yt-dlp",
			FFmpegPath:                "/usr/bin/ffmpeg",
			LLMTranslationEnabled:     false,
			LLMTranslationBatchSize:   25,
			LLMTranslationMaxWorkers:  3,
			LLMTranslationContextSize: 2,
			LLMTranslationSourceLang:  "en",
			LLMTranslationTargetLang:  "zh-Hans",
			TTSEnabled:                false,
			MaxConcurrent:             4,
		},
		AgentOpenAPI: AgentOpenAPIConfig{
			Enabled:          false,
			BaseURL:          "https://open.ytb2bili.com/agent/v1",
			DefaultRateLimit: 60,
		},
		Agent: agent.DefaultConfig(),
		LLM: LLMConfig{
			Provider:    "openai",
			BaseURL:     "https://api.openai.com",
			Model:       "",
			Models:      []string{"gpt-4o-mini"},
			Temperature: 0.7,
			MaxTokens:   4096,
			Timeout:     120,
		},
		Deepseek: DeepseekConfig{
			APIKey: "",
		},
	}
}

func NewDefaultConfig() *AppConfig {
	return getDefaultConfig()
}

func LoadConfig(path string) (*AppConfig, error) {
	configFile := resolveConfigPath(path)
	cfg := NewDefaultConfig()

	if _, err := os.Stat(configFile); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		if err := SaveConfigToPath(configFile, cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	}

	if _, err := toml.DecodeFile(configFile, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func SaveConfig(config *AppConfig) error {
	return SaveConfigToPath(resolveConfigPath(""), config)
}

func SaveConfigToPath(path string, config *AppConfig) error {
	var buf bytes.Buffer
	encoder := toml.NewEncoder(&buf)
	if err := encoder.Encode(config); err != nil {
		return err
	}

	return os.WriteFile(path, buf.Bytes(), 0o644)
}

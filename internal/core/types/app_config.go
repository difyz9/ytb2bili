package types

import (
	"bytes"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// AppConfig 应用程序配置
type AppConfig struct {
	Path        string        `toml:"-"`
	Listen      string        `toml:"listen"`
	Environment string        `toml:"environment"`
	Debug       bool          `toml:"debug"`
	Database    Database      `toml:"database"`
	Auth        AuthConfig    `toml:"auth"`
	AppAuth     AppAuthConfig `toml:"app_auth"` // 应用启动认证配置
	FileUpDir   string        `toml:"fileUpDir"`
	YtDlpPath   string        `toml:"yt_dlp_path"` // yt-dlp 安装路径

	TenCosConfig        *TencentCosConfig    `toml:"TenCosConfig"`        // 腾讯云 COS 存储配置
	BaiduTransConfig    *BaiduTransConfig    `toml:"BaiduTransConfig"`    // 百度翻译服务配置
	DeepSeekTransConfig *DeepSeekTransConfig `toml:"DeepSeekTransConfig"` // DeepSeek翻译服务配置
	TranslatorConfig    *TranslatorConfig    `toml:"TranslatorConfig"`    // 翻译器总配置
	ProxyConfig         *ProxyConfig         `toml:"ProxyConfig"`         // 代理配置
	AnalyticsConfig     *AnalyticsConfig     `toml:"AnalyticsConfig"`     // 数据分析配置
	BilibiliConfig      *BilibiliConfig      `toml:"BilibiliConfig"`      // Bilibili上传配置
}

// BilibiliConfig Bilibili上传配置
type BilibiliConfig struct {
	Copyright          int    `toml:"copyright"`            // 1=自制, 2=转载
	Source             string `toml:"source"`               // 转载来源（当 Copyright=2 时必填）
	NoReprint          int    `toml:"no_reprint"`           // 0=允许转载, 1=禁止转载
	UseOriginalTitle   bool   `toml:"use_original_title"`   // true=使用原视频标题, false=使用AI生成标题
	UseOriginalDesc    bool   `toml:"use_original_desc"`    // true=使用原视频描述, false=使用AI生成描述
	CustomDescTemplate string `toml:"custom_desc_template"` // 自定义描述模板，支持变量: {original_desc}, {ai_desc}
}

type TencentCosConfig struct {
	Enabled      bool // 是否启用腾讯云 COS 存储
	CosBucketURL string
	CosSecretId  string
	CosSecretKey string
	CosRegion    string
	CosBucket    string
	SubAppId     string
	CosUrL       string
}

// Database 数据库配置
type Database struct {
	Type     string `toml:"type"`     // postgres, mysql, sqlite
	Host     string `toml:"host"`     // 对于 sqlite，这是数据库文件路径
	Port     int    `toml:"port"`     // sqlite 不需要
	Username string `toml:"username"` // sqlite 不需要
	Password string `toml:"password"` // sqlite 不需要
	Database string `toml:"database"` // 对于 sqlite，这是文件名
	SSLMode  string `toml:"ssl_mode"` // sqlite 不需要
	Timezone string `toml:"timezone"`
}

// AuthConfig 认证配置
type AuthConfig struct {
	JWTSecret     string `toml:"jwt_secret"`
	JWTExpiration int    `toml:"jwt_expiration"` // 小时
	SessionSecret string `toml:"session_secret"`
}

// AppAuthConfig 应用启动认证配置
type AppAuthConfig struct {
	Enabled       bool   `toml:"enabled"`        // 是否启用应用认证
	APIURL        string `toml:"api_url"`        // 认证API地址
	AppID         string `toml:"app_id"`         // 应用ID
	AppSecret     string `toml:"app_secret"`     // 应用密钥
	CheckInterval int    `toml:"check_interval"` // 定期检查间隔（分钟），0表示只在启动时检查
	SkipOnError   bool   `toml:"skip_on_error"`  // 认证失败时是否跳过（开发环境可设置为true）
}

// GetDSN 获取数据库连接字符串
func (d Database) GetDSN() string {
	switch d.Type {
	case "postgres", "postgresql":
		return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=%s",
			d.Host, d.Username, d.Password, d.Database, d.Port, d.SSLMode, d.Timezone)
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			d.Username, d.Password, d.Host, d.Port, d.Database)
	case "sqlite", "sqlite3":
		// SQLite 数据库文件路径
		if d.Host != "" {
			// 如果指定了 host，使用 host 作为完整路径
			return d.Host
		}
		// 否则使用 database 作为文件名，存储在当前目录
		if d.Database == "" {
			d.Database = "bili_up.db"
		}
		return d.Database
	default:
		return ""
	}
}

// BaiduTransConfig 百度翻译服务配置
type BaiduTransConfig struct {
	Enabled   bool   `toml:"enabled"`    // 是否启用翻译服务
	AppId     string `toml:"app_id"`     // 百度翻译AppID
	SecretKey string `toml:"secret_key"` // 百度翻译密钥
	Endpoint  string `toml:"endpoint"`   // API端点
}

// DeepSeekTransConfig DeepSeek翻译服务配置
type DeepSeekTransConfig struct {
	Enabled   bool   `toml:"enabled"`    // 是否启用翻译服务
	ApiKey    string `toml:"api_key"`    // DeepSeek API密钥
	Model     string `toml:"models"`     // 使用的模型，默认为 deepseek-chat
	Endpoint  string `toml:"endpoint"`   // API端点，默认为 https://api.deepseek.com
	Timeout   int    `toml:"timeout"`    // 超时时间（秒）
	MaxTokens int    `toml:"max_tokens"` // 最大token数
}

// TranslatorConfig 翻译器总配置
type TranslatorConfig struct {
	DefaultProvider   string   `toml:"default_provider"`   // 默认翻译提供商
	FallbackProviders []string `toml:"fallback_providers"` // 备选翻译提供商
	MaxRetries        int      `toml:"max_retries"`        // 最大重试次数
	Timeout           int      `toml:"timeout"`            // 超时时间（秒）
	EnableCache       bool     `toml:"enable_cache"`       // 是否启用缓存
	CacheExpiry       int      `toml:"cache_expiry"`       // 缓存过期时间（秒）
}

// ProxyConfig 代理配置
type ProxyConfig struct {
	UseProxy  bool   `toml:"use_proxy"`  // 是否使用代理
	ProxyHost string `toml:"proxy_host"` // 代理地址 (例如: http://127.0.0.1:7890)
}

// AnalyticsConfig 数据分析配置
type AnalyticsConfig struct {
	Enabled       bool   `toml:"enabled"`        // 是否启用数据分析
	ServerURL     string `toml:"server_url"`     // 分析服务器地址
	APIKey        string `toml:"api_key"`        // API密钥
	ProductID     string `toml:"product_id"`     // 产品ID
	Debug         bool   `toml:"debug"`          // 是否启用调试模式
	EncryptionKey string `toml:"encryption_key"` // AES加密密钥（可选，16/24/32字节）
}

// NewDefaultConfig 创建默认配置
func NewDefaultConfig() *AppConfig {
	return &AppConfig{
		Listen:      ":8096",
		Environment: "development",
		Debug:       true,
		Database: Database{
			Type:     "postgres",
			Host:     "localhost",
			Port:     5432,
			Username: "postgres",
			Password: "password",
			Database: "bili_up_db",
			SSLMode:  "disable",
			Timezone: "Asia/Shanghai",
		},

		Auth: AuthConfig{
			JWTSecret:     "your-jwt-secret-key",
			JWTExpiration: 24,
			SessionSecret: "your-session-secret",
		},

		// 腾讯云 COS 配置（默认值，可被 config.toml 覆盖）
		TenCosConfig: &TencentCosConfig{
			Enabled:      false,
			CosBucketURL: "",
			CosSecretId:  "",
			CosSecretKey: "",
			CosRegion:    "",
			CosBucket:    "",
			SubAppId:     "",
			CosUrL:       "",
		},

		// DeepSeek 翻译配置（默认值，可被 config.toml 覆盖）
		DeepSeekTransConfig: &DeepSeekTransConfig{
			Enabled:   false,
			ApiKey:    "",
			Model:     "deepseek-chat",
			Endpoint:  "https://api.deepseek.com",
			Timeout:   60,
			MaxTokens: 4000,
		},

		// 代理配置（默认值，可被 config.toml 覆盖）
		ProxyConfig: &ProxyConfig{
			UseProxy:  false,
			ProxyHost: "",
		},

		// 数据分析配置（默认值，可被 config.toml 覆盖）
		AnalyticsConfig: &AnalyticsConfig{
			Enabled:   false,
			ServerURL: "http://localhost:8080",
			APIKey:    "",
			ProductID: "bili-up-api",
			Debug:     false,
		},

		// Bilibili 配置（默认值，可被 config.toml 覆盖）
		BilibiliConfig: &BilibiliConfig{
			Copyright:          1, // 默认自制
			Source:             "",
			NoReprint:          1,     // 默认禁止转载
			UseOriginalTitle:   true,  // 默认使用原视频标题
			UseOriginalDesc:    false, // 默认使用AI生成的描述
			CustomDescTemplate: "",    // 默认不使用自定义模板
		},
	}
}

// LoadConfig 加载配置
func LoadConfig(configFile string) (*AppConfig, error) {
	// 先创建默认配置（包含所有硬编码的配置）
	config := NewDefaultConfig()
	config.Path = configFile

	// 检查配置文件是否存在
	_, err := os.Stat(configFile)
	if err != nil {
		// 如果文件不存在，不创建默认配置文件，使用硬编码配置即可
		return config, nil
	}

	// 创建临时结构体用于读取 config.toml（只包含可配置字段）
	var fileConfig struct {
		Listen              string               `toml:"listen"`
		Environment         string               `toml:"environment"`
		Debug               bool                 `toml:"debug"`
		Database            Database             `toml:"database"`
		Auth                AuthConfig           `toml:"auth"`
		FileUpDir           string               `toml:"fileUpDir"`
		YtDlpPath           string               `toml:"yt_dlp_path"`
		TenCosConfig        *TencentCosConfig    `toml:"TenCosConfig"`
		DeepSeekTransConfig *DeepSeekTransConfig `toml:"DeepSeekTransConfig"`
		ProxyConfig         *ProxyConfig         `toml:"ProxyConfig"`
		AnalyticsConfig     *AnalyticsConfig     `toml:"AnalyticsConfig"`
		BilibiliConfig      *BilibiliConfig      `toml:"BilibiliConfig"`
	}

	// 解码TOML配置文件
	_, err = toml.DecodeFile(configFile, &fileConfig)
	if err != nil {
		return nil, err
	}

	// 只覆盖配置文件中存在的字段，保留硬编码的配置
	config.Listen = fileConfig.Listen
	config.Environment = fileConfig.Environment
	config.Debug = fileConfig.Debug
	config.Database = fileConfig.Database
	config.Auth = fileConfig.Auth
	config.FileUpDir = fileConfig.FileUpDir
	config.YtDlpPath = fileConfig.YtDlpPath
	if fileConfig.TenCosConfig != nil {
		config.TenCosConfig = fileConfig.TenCosConfig
	}
	if fileConfig.DeepSeekTransConfig != nil {
		config.DeepSeekTransConfig = fileConfig.DeepSeekTransConfig
	}
	if fileConfig.ProxyConfig != nil {
		config.ProxyConfig = fileConfig.ProxyConfig
	}
	if fileConfig.AnalyticsConfig != nil {
		config.AnalyticsConfig = fileConfig.AnalyticsConfig
	}
	if fileConfig.BilibiliConfig != nil {
		config.BilibiliConfig = fileConfig.BilibiliConfig
	}

	return config, nil
}

// SaveConfig 保存配置（只保存可配置字段，不保存硬编码配置）
func SaveConfig(config *AppConfig) error {
	// 只保存用户可配置的字段
	fileConfig := struct {
		Listen              string               `toml:"listen"`
		Environment         string               `toml:"environment"`
		Debug               bool                 `toml:"debug"`
		Database            Database             `toml:"database"`
		Auth                AuthConfig           `toml:"auth"`
		FileUpDir           string               `toml:"fileUpDir"`
		YtDlpPath           string               `toml:"yt_dlp_path"`
		TenCosConfig        *TencentCosConfig    `toml:"TenCosConfig"`
		DeepSeekTransConfig *DeepSeekTransConfig `toml:"DeepSeekTransConfig"`
		ProxyConfig         *ProxyConfig         `toml:"ProxyConfig"`
		AnalyticsConfig     *AnalyticsConfig     `toml:"AnalyticsConfig"`
		BilibiliConfig      *BilibiliConfig      `toml:"BilibiliConfig"`
	}{
		Listen:              config.Listen,
		Environment:         config.Environment,
		Debug:               config.Debug,
		Database:            config.Database,
		Auth:                config.Auth,
		FileUpDir:           config.FileUpDir,
		YtDlpPath:           config.YtDlpPath,
		TenCosConfig:        config.TenCosConfig,
		DeepSeekTransConfig: config.DeepSeekTransConfig,
		ProxyConfig:         config.ProxyConfig,
		AnalyticsConfig:     config.AnalyticsConfig,
		BilibiliConfig:      config.BilibiliConfig,
	}

	buf := new(bytes.Buffer)

	// 写入注释说明
	buf.WriteString("# Bilibili 视频上传后端 - 配置文件\n\n")
	buf.WriteString("# 注意：以下配置已硬编码在代码中，无需在此配置：\n")
	buf.WriteString("# - BaiduTransConfig (百度翻译)\n")
	buf.WriteString("# - app_auth (应用认证)\n")
	buf.WriteString("# \n")
	buf.WriteString("# 所有配置都可以通过 config.toml 或 API 接口动态配置\n\n")

	encoder := toml.NewEncoder(buf)
	if err := encoder.Encode(&fileConfig); err != nil {
		return err
	}

	return os.WriteFile(config.Path, buf.Bytes(), 0644)
}

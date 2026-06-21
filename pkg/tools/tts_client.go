package tools

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	appconfig "github.com/zolagz/ytb2bili/internal/config"
	storemodel "github.com/zolagz/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Default TTS configuration constants shared across packages.
const (
	DefaultTTSProvider       = "auto"
	DefaultTTSVoice          = "zh-CN-XiaoxiaoNeural"
	DefaultTTSLocale         = "zh-CN"
	DefaultTTSAudioFormat    = "audio-24khz-96kbitrate-mono-mp3"
	DefaultTTSTimeoutSeconds = 180
	DefaultTTSRate           = 1
	DefaultTTSVolume         = 100
)

// TTSConfig holds the runtime TTS configuration.
type TTSConfig struct {
	BaseURL             string
	APIKey              string
	ProjectID           string
	AzureSubscriptionKey string
	AzureRegion         string
	TencentSecretID     string
	TencentSecretKey    string
	TencentRegion       string
	appconfig.TTSConfig
}

// TTSClient TTS 客户端工具 — 通过 TTSEngine 插件体系调用各类 TTS 供应商。
// 支持的 provider：azure, edge, openai, tencent
type TTSClient struct {
	config       TTSConfig
	db           *gorm.DB
	httpClient   *http.Client
	logger       *zap.Logger
	profileCache map[string]ttsProfile
	mu           sync.Mutex
	engines      *ttsEngineRegistry // 引擎注册表
}

// TTSRequest TTS 请求
type TTSRequest struct {
	UserID           string
	Text             string
	Provider         string
	VoiceName        string
	Language         string
	Format           string
	Search           string
	Rate             float64
	Volume           float64
	Pitch            float64
	StorageKey       string
	DownloadFilename string
	LocalOutputPath  string
}

// TTSRespData TTS 响应数据
type TTSRespData struct {
	Text      string
	Audio     []byte
	AudioURL  string
	Provider  string
	Voice     string
	Locale    string
	Format    string
	FileSize  int64
	CosKey    string
	CosURL    string
	LocalPath string
	TaskID    string
	Status    string
	Charged   string
}

type ttsProfile struct {
	Provider string
	Voice    string
	Locale   string
}

type storedTTSConfig struct {
	ProjectID      string   `json:"project_id"`
	Provider       string   `json:"provider"`
	Search         string   `json:"search"`
	VoiceName      string   `json:"voice_name"`
	Voice          string   `json:"voice"`
	Language       string   `json:"language"`
	Locale         string   `json:"locale"`
	Format         string   `json:"format"`
	Rate           *float64 `json:"rate"`
	Volume         *float64 `json:"volume"`
	Pitch          *float64 `json:"pitch"`
	TimeoutSeconds *int     `json:"timeout_seconds"`
}

// NewTTSClient 创建 TTS 客户端
func NewTTSClient(cfg *appconfig.AppConfig, db *gorm.DB, logger *zap.Logger) *TTSClient {
	config := buildRuntimeTTSConfig(cfg)
	return newTTSClientWithConfig(config, db, logger)
}

func newTTSClientWithConfig(config TTSConfig, db *gorm.DB, logger *zap.Logger) *TTSClient {
	config = normalizeTTSConfig(config)

	// 初始化引擎注册表
	engines := newTTSEngineRegistry()

	// Azure 引擎（需要 subscription_key + region）
	if key := strings.TrimSpace(config.AzureSubscriptionKey); key != "" && strings.TrimSpace(config.AzureRegion) != "" {
		engines.Register(NewAzureTTSEngine(key, strings.TrimSpace(config.AzureRegion)))
	}

	// Edge-TTS 引擎（免费，无需 API Key）
	engines.Register(NewEdgeTTSEngine())

	// OpenAI TTS 引擎（需要 API Key）
	if key := strings.TrimSpace(config.APIKey); key != "" {
		engines.Register(NewOpenAITTSEngine(key, config.BaseURL, ""))
	}

	// 腾讯云引擎（需要 secret_id + secret_key）
	if key := strings.TrimSpace(config.TencentSecretID); key != "" && strings.TrimSpace(config.TencentSecretKey) != "" {
		engines.Register(NewTencentTTSEngine(key, strings.TrimSpace(config.TencentSecretKey), strings.TrimSpace(config.TencentRegion)))
	}

	return &TTSClient{
		config:       config,
		db:           db,
		httpClient:   &http.Client{Timeout: 60 * time.Second},
		logger:       logger,
		profileCache: make(map[string]ttsProfile),
		engines:      engines,
	}
}

func buildRuntimeTTSConfig(cfg *appconfig.AppConfig) TTSConfig {
	if cfg == nil {
		return normalizeTTSConfig(TTSConfig{})
	}
	return normalizeTTSConfig(TTSConfig{
		BaseURL:              cfg.LLMBaseURL(),
		APIKey:               cfg.LLMAPIKey(),
		ProjectID:            "",
		AzureSubscriptionKey: cfg.AzureTTS.SubscriptionKey,
		AzureRegion:          cfg.AzureTTS.Region,
		TencentSecretID:      "",
		TencentSecretKey:     "",
		TencentRegion:        "ap-guangzhou",
		TTSConfig:            cfg.GetTTSConfig(),
	})
}

// SynthesizeSpeech 合成语音 — 直接调用 Azure / Tencent Cloud API
func (c *TTSClient) SynthesizeSpeech(ctx context.Context, req TTSRequest) (*TTSRespData, error) {
	effectiveConfig, err := c.resolveRequestConfig(ctx, req)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(req.Text) == "" {
		return nil, fmt.Errorf("TTS text is empty")
	}

	profile, err := c.resolveProfile(ctx, req, effectiveConfig)
	if err != nil {
		return nil, err
	}

	requestCtx := ctx
	var cancel context.CancelFunc
	if timeout := effectiveConfig.TimeoutDuration(); timeout > 0 {
		requestCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	rate := c.pickFloat(req.Rate, effectiveConfig.Rate)
	volume := clampTTSVolume(c.pickFloat(req.Volume, effectiveConfig.Volume))
	pitch := c.pickFloat(req.Pitch, effectiveConfig.Pitch)

	engine, engineErr := c.engines.Get(profile.Provider)
	if engineErr != nil {
		return nil, engineErr
	}
	audio, err := engine.Synthesize(requestCtx, strings.TrimSpace(req.Text), profile.Voice, rate, volume, pitch)
	if err != nil {
		return nil, fmt.Errorf("TTS synthesis failed (%s): %w", profile.Provider, err)
	}

	localPath := strings.TrimSpace(req.LocalOutputPath)
	if localPath != "" {
		if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
			return nil, fmt.Errorf("failed to create subtitle audio directory: %w", err)
		}
		if err := os.WriteFile(localPath, audio, 0o644); err != nil {
			return nil, fmt.Errorf("failed to save subtitle audio file: %w", err)
		}
	}

	return &TTSRespData{
		Text:      strings.TrimSpace(req.Text),
		Audio:     audio,
		Provider:  profile.Provider,
		Voice:     profile.Voice,
		Locale:    profile.Locale,
		Format:    c.pickString(req.Format, effectiveConfig.Format),
		FileSize:  int64(len(audio)),
		LocalPath: localPath,
		Status:    "completed",
	}, nil
}

// synthesizeAzure calls the Azure Cognitive Services TTS REST API directly.
func (c *TTSClient) synthesizeAzure(ctx context.Context, config TTSConfig, profile ttsProfile, text string, rate, volume, pitch float64) ([]byte, error) {
	subKey := strings.TrimSpace(config.AzureSubscriptionKey)
	region := strings.TrimSpace(config.AzureRegion)
	if subKey == "" || region == "" {
		return nil, fmt.Errorf("Azure TTS requires subscription_key and region; configure [azure_tts] in config.toml or user settings")
	}

	endpoint := fmt.Sprintf("https://%s.tts.speech.microsoft.com/cognitiveservices/v1", region)

	// Build SSML
	ssml := fmt.Sprintf(
		`<speak version='1.0' xml:lang='%s' xmlns='http://www.w3.org/2001/10/synthesis' xmlns:mstts='http://www.w3.org/2001/mstts'>`+
			`<voice name='%s'>`+
			`<prosody rate='%.1f' volume='%.0f' pitch='%+.0fHz'>`+
			`%s`+
			`</prosody>`+
			`</voice>`+
			`</speak>`,
		profile.Locale, profile.Voice, rate, volume, pitch, escapeSSML(text),
	)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(ssml))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Ocp-Apim-Subscription-Key", subKey)
	req.Header.Set("Content-Type", "application/ssml+xml")
	req.Header.Set("X-Microsoft-OutputFormat", "audio-16khz-128kbitrate-mono-mp3")
	req.Header.Set("User-Agent", "ytb2bili-tts/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Azure TTS request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Azure TTS returned status %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// synthesizeTencent calls the Tencent Cloud TTS API directly using TC3-HMAC-SHA256 signing.
func (c *TTSClient) synthesizeTencent(ctx context.Context, config TTSConfig, profile ttsProfile, text string, rate, volume, pitch float64) ([]byte, error) {
	secretID := strings.TrimSpace(config.TencentSecretID)
	secretKey := strings.TrimSpace(config.TencentSecretKey)
	region := strings.TrimSpace(config.TencentRegion)

	if secretID == "" || secretKey == "" {
		return nil, fmt.Errorf("Tencent TTS requires secret_id and secret_key; configure [tencent_tts] in config.toml or user settings")
	}
	if region == "" {
		region = "ap-guangzhou"
	}

	// Map voice name to Tencent VoiceType (integer)
	voiceType := resolveTencentVoiceType(profile.Voice)

	// Speed: Tencent uses -2 to 2, map from our 0.5-2.0 range
	speed := rate

	body := map[string]interface{}{
		"Text":           text,
		"SessionId":      fmt.Sprintf("ytb2bili-%d", time.Now().UnixNano()),
		"VoiceType":      voiceType,
		"PrimaryLanguage": 1, // 1 = Chinese
		"Codec":          "mp3",
		"Speed":          speed,
		"Volume":         volume / 100.0, // Tencent uses 0-1
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	host := "tts.tencentcloudapi.com"
	service := "tts"
	action := "TextToVoice"
	version := "2019-08-23"
	algorithm := "TC3-HMAC-SHA256"
	timestamp := time.Now().Unix()

	req, err := http.NewRequestWithContext(ctx, "POST", "https://"+host, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	// Build TC3 signature
	httpRequestMethod := "POST"
	canonicalURI := "/"
	canonicalQueryString := ""
	canonicalHeaders := fmt.Sprintf("content-type:application/json\nhost:%s\n", host)
	signedHeaders := "content-type;host"
	payloadHash := sha256Hex(bodyBytes)
	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		httpRequestMethod, canonicalURI, canonicalQueryString, canonicalHeaders, signedHeaders, payloadHash)

	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")
	credentialScope := fmt.Sprintf("%s/%s/tc3_request", date, service)
	stringToSign := fmt.Sprintf("%s\n%d\n%s\n%s",
		algorithm, timestamp, credentialScope, sha256Hex([]byte(canonicalRequest)))

	secretDate := hmacSHA256([]byte("TC3"+secretKey), []byte(date))
	secretService := hmacSHA256(secretDate, []byte(service))
	secretSigning := hmacSHA256(secretService, []byte("tc3_request"))
	signature := hex.EncodeToString(hmacSHA256(secretSigning, []byte(stringToSign)))

	authorization := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm, secretID, credentialScope, signedHeaders, signature)

	req.Header.Set("Authorization", authorization)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Host", host)
	req.Header.Set("X-TC-Action", action)
	req.Header.Set("X-TC-Version", version)
	req.Header.Set("X-TC-Timestamp", fmt.Sprintf("%d", timestamp))
	req.Header.Set("X-TC-Region", region)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Tencent TTS request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Response struct {
			Audio   string `json:"Audio"`
			Error   struct {
				Code    string `json:"Code"`
				Message string `json:"Message"`
			} `json:"Error"`
		} `json:"Response"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse Tencent TTS response: %w", err)
	}
	if result.Response.Error.Code != "" {
		return nil, fmt.Errorf("Tencent TTS error: %s - %s", result.Response.Error.Code, result.Response.Error.Message)
	}

	// Tencent returns base64-encoded audio
	audio, err := base64Decode(result.Response.Audio)
	if err != nil {
		return nil, fmt.Errorf("failed to decode Tencent TTS audio: %w", err)
	}

	return audio, nil
}

// SynthesizeSubtitleAudio 合成字幕音频（带自动文件名生成）
func (c *TTSClient) SynthesizeSubtitleAudio(ctx context.Context, userID, text, videoID string, index int, localAudioDir string, speechConfig interface {
	GetProvider() string
	GetLanguage() string
	GetVoiceName() string
	GetFormat() string
	GetSearch() string
	GetRate() float64
	GetVolume() float64
	GetPitch() float64
}) (*TTSRespData, error) {
	fileName := fmt.Sprintf("index_%04d.mp3", index)
	storageKey := fmt.Sprintf("%s/%s", strings.Trim(strings.TrimSpace(videoID), "/"), fileName)
	localPath := ""
	if strings.TrimSpace(localAudioDir) != "" {
		localPath = filepath.Join(strings.TrimSpace(localAudioDir), fileName)
	}

	req := TTSRequest{
		UserID:           strings.TrimSpace(userID),
		Text:             text,
		StorageKey:       storageKey,
		DownloadFilename: fileName,
		LocalOutputPath:  localPath,
	}
	if speechConfig != nil {
		if provider := speechConfig.GetProvider(); provider != "" {
			req.Provider = provider
		}
		if voiceName := speechConfig.GetVoiceName(); voiceName != "" {
			req.VoiceName = voiceName
		}
		if language := speechConfig.GetLanguage(); language != "" {
			req.Language = language
		}
		if format := speechConfig.GetFormat(); format != "" {
			req.Format = format
		}
		if search := speechConfig.GetSearch(); search != "" {
			req.Search = search
		}
		if rate := speechConfig.GetRate(); rate != 0 {
			req.Rate = rate
		}
		if volume := speechConfig.GetVolume(); volume != 0 {
			req.Volume = volume
		}
		if pitch := speechConfig.GetPitch(); pitch != 0 {
			req.Pitch = pitch
		}
	}
	if req.Search == "" {
		req.Search = req.VoiceName
	}

	c.logger.Info("合成字幕音频",
		zap.String("user_id", req.UserID),
		zap.String("videoID", videoID),
		zap.Int("index", index),
		zap.String("storage_key", storageKey),
		zap.String("local_path", localPath),
		zap.String("provider_override", req.Provider),
		zap.String("voice_name", req.VoiceName),
		zap.String("locale", req.Language),
		zap.Int("textLength", len(text)))

	return c.SynthesizeSpeech(ctx, req)
}

// resolveProfile resolves which provider and voice to use.
func (c *TTSClient) resolveProfile(ctx context.Context, req TTSRequest, config TTSConfig) (ttsProfile, error) {
	requestedProvider := strings.ToLower(strings.TrimSpace(c.pickString(req.Provider, config.Provider)))
	requestedVoice := strings.TrimSpace(c.pickString(req.VoiceName, config.Voice))
	locale := strings.TrimSpace(c.pickString(req.Language, config.Locale))
	search := strings.TrimSpace(c.pickString(req.Search, config.Search))
	if search == "" {
		search = requestedVoice
	}
	if requestedProvider == "" {
		requestedProvider = DefaultTTSProvider
	}
	if locale == "" {
		locale = DefaultTTSLocale
	}

	if requestedProvider != "auto" {
		// Use the specified provider directly
		return c.resolveVoiceForProvider(requestedProvider, locale, requestedVoice, search)
	}

	// Auto mode: try providers based on available credentials
	return c.resolveAutoProfile(config, locale, requestedVoice, search)
}

func (c *TTSClient) resolveAutoProfile(config TTSConfig, locale, requestedVoice, search string) (ttsProfile, error) {
	// Try Azure first if credentials are available
	if strings.TrimSpace(config.AzureSubscriptionKey) != "" && strings.TrimSpace(config.AzureRegion) != "" {
		profile, err := c.resolveVoiceForProvider("azure", locale, requestedVoice, search)
		if err == nil {
			return profile, nil
		}
	}

	// Try Tencent if credentials are available
	if strings.TrimSpace(config.TencentSecretID) != "" && strings.TrimSpace(config.TencentSecretKey) != "" {
		profile, err := c.resolveVoiceForProvider("tencent", locale, requestedVoice, search)
		if err == nil {
			return profile, nil
		}
	}

	return ttsProfile{}, fmt.Errorf("no available TTS provider; configure [azure_tts] or [tencent_tts] in config.toml with valid credentials")
}

// resolveVoiceForProvider resolves a voice for the given provider using the embedded catalog.
func (c *TTSClient) resolveVoiceForProvider(provider, locale, requestedVoice, search string) (ttsProfile, error) {
	// Use the embedded voice catalog (loaded from ttsdata/*.json)
	voices, err := c.getVoicesForProvider(provider, locale)
	if err != nil {
		return ttsProfile{}, err
	}

	if requestedVoice != "" {
		for _, voice := range voices {
			if strings.EqualFold(strings.TrimSpace(voice.ShortName), requestedVoice) {
				return ttsProfile{Provider: provider, Voice: voice.ShortName, Locale: c.pickString(strings.TrimSpace(voice.Locale), locale)}, nil
			}
		}
		// Requested voice not found; try search
		if search != "" {
			for _, voice := range voices {
				if strings.Contains(strings.ToLower(voice.ShortName), strings.ToLower(search)) ||
					strings.Contains(strings.ToLower(voice.DisplayName), strings.ToLower(search)) {
					return ttsProfile{Provider: provider, Voice: voice.ShortName, Locale: c.pickString(strings.TrimSpace(voice.Locale), locale)}, nil
				}
			}
		}
	}

	if len(voices) == 0 {
		return ttsProfile{}, fmt.Errorf("provider %s returned no voices for locale %s", provider, locale)
	}

	voice := voices[0]
	return ttsProfile{Provider: provider, Voice: strings.TrimSpace(voice.ShortName), Locale: c.pickString(strings.TrimSpace(voice.Locale), locale)}, nil
}

// VoiceInfo is a minimal voice record for matching.
type VoiceInfo struct {
	ShortName   string `json:"ShortName"`
	DisplayName string `json:"DisplayName"`
	Locale      string `json:"Locale"`
}

// getVoicesForProvider returns voices from the embedded catalog for a provider and locale.
func (c *TTSClient) getVoicesForProvider(provider, locale string) ([]VoiceInfo, error) {
	// Read from the embedded catalog files
	catalog, err := loadEmbeddedVoiceCatalog(provider, locale)
	if err != nil {
		return nil, fmt.Errorf("load voice catalog for %s: %w", provider, err)
	}

	var result []VoiceInfo
	for _, v := range catalog {
		// Filter by locale if specified
		if locale != "" && !strings.HasPrefix(strings.ToLower(v.Locale), strings.ToLower(locale)) {
			continue
		}
		result = append(result, VoiceInfo{
			ShortName:   v.ShortName,
			DisplayName: v.DisplayName,
			Locale:      v.Locale,
		})
	}

	return result, nil
}

// resolveRequestConfig merges user-level TTS settings into the effective config.
func (c *TTSClient) resolveRequestConfig(ctx context.Context, req TTSRequest) (TTSConfig, error) {
	effectiveConfig := c.config
	userID := strings.TrimSpace(req.UserID)
	if c.db != nil && userID != "" {
		userConfig, err := c.loadUserTTSConfig(ctx, userID)
		if err != nil {
			return TTSConfig{}, err
		}
		effectiveConfig = c.mergeConfig(effectiveConfig, userConfig)

		// Also check user-level credentials
		settings, err := c.loadUserSettings(ctx, userID)
		if err == nil {
			if v := strings.TrimSpace(settings[storemodel.UserSettingKeyAzureTTSSubscriptionKey]); v != "" {
				effectiveConfig.AzureSubscriptionKey = v
			}
			if v := strings.TrimSpace(settings[storemodel.UserSettingKeyAzureTTSRegion]); v != "" {
				effectiveConfig.AzureRegion = v
			}
			if v := strings.TrimSpace(settings[storemodel.UserSettingKeyTencentTTSSecretID]); v != "" {
				effectiveConfig.TencentSecretID = v
			}
			if v := strings.TrimSpace(settings[storemodel.UserSettingKeyTencentTTSSecretKey]); v != "" {
				effectiveConfig.TencentSecretKey = v
			}
			if v := strings.TrimSpace(settings[storemodel.UserSettingKeyTencentTTSRegion]); v != "" {
				effectiveConfig.TencentRegion = v
			}
		}
	}

	return normalizeTTSConfig(effectiveConfig), nil
}

func (c *TTSClient) loadUserSettings(ctx context.Context, userID string) (map[string]string, error) {
	if c.db == nil || userID == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var record storemodel.UserSettings
	err := c.db.WithContext(ctx).Where("user_id = ?", userID).First(&record).Error
	if err != nil {
		return nil, err
	}
	return record.ToSettingsMap(), nil
}

func (c *TTSClient) loadUserTTSConfig(ctx context.Context, userID string) (TTSConfig, error) {
	var record storemodel.UserSettings
	err := c.db.WithContext(ctx).Where("user_id = ?", userID).First(&record).Error
	if err == nil {
		value := storemodel.ResolveSubtitleAudioTTSConfigValue(record.ToSettingsMap())
		config, ok := parseStoredUserTTSConfig(value)
		if !ok {
			if c.logger != nil && strings.TrimSpace(value) != "" {
				c.logger.Warn("用户 TTS 配置格式无效，回退默认配置",
					zap.String("user_id", userID),
					zap.String("setting_key", storemodel.UserSettingKeySubtitleAudioTTSConfig))
			}
			return TTSConfig{}, nil
		}
		return config, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return TTSConfig{}, nil
	}
	return TTSConfig{}, fmt.Errorf("query user tts settings: %w", err)
}

func (c *TTSClient) mergeConfig(baseConfig, overrideConfig TTSConfig) TTSConfig {
	merged := baseConfig
	if value := strings.TrimSpace(overrideConfig.Provider); value != "" {
		merged.Provider = strings.ToLower(value)
	}
	if value := strings.TrimSpace(overrideConfig.Search); value != "" {
		merged.Search = value
	}
	if value := strings.TrimSpace(overrideConfig.Voice); value != "" {
		merged.Voice = value
	}
	if value := strings.TrimSpace(overrideConfig.Locale); value != "" {
		merged.Locale = value
	}
	if value := strings.TrimSpace(overrideConfig.Format); value != "" {
		merged.Format = value
	}
	if overrideConfig.TimeoutSeconds > 0 {
		merged.TimeoutSeconds = overrideConfig.TimeoutSeconds
	}
	if overrideConfig.Rate != 0 {
		merged.Rate = overrideConfig.Rate
	}
	if overrideConfig.Volume != 0 {
		merged.Volume = overrideConfig.Volume
	}
	if overrideConfig.Pitch != 0 {
		merged.Pitch = overrideConfig.Pitch
	}
	return normalizeTTSConfig(merged)
}

func normalizeTTSConfig(config TTSConfig) TTSConfig {
	config.APIKey = strings.TrimSpace(config.APIKey)
	config.ProjectID = strings.TrimSpace(config.ProjectID)
	config.AzureSubscriptionKey = strings.TrimSpace(config.AzureSubscriptionKey)
	config.AzureRegion = strings.TrimSpace(config.AzureRegion)
	config.TencentSecretID = strings.TrimSpace(config.TencentSecretID)
	config.TencentSecretKey = strings.TrimSpace(config.TencentSecretKey)
	config.TencentRegion = strings.TrimSpace(config.TencentRegion)
	config.Provider = strings.ToLower(strings.TrimSpace(config.Provider))
	config.Search = strings.TrimSpace(config.Search)
	config.Voice = strings.TrimSpace(config.Voice)
	config.Locale = strings.TrimSpace(config.Locale)
	config.Format = strings.TrimSpace(config.Format)
	if config.TimeoutSeconds <= 0 {
		config.TimeoutSeconds = DefaultTTSTimeoutSeconds
	}
	if config.Provider == "" {
		config.Provider = DefaultTTSProvider
	}
	if config.Voice == "" {
		config.Voice = DefaultTTSVoice
	}
	if config.Locale == "" {
		config.Locale = DefaultTTSLocale
	}
	if config.Format == "" {
		config.Format = DefaultTTSAudioFormat
	}
	if config.Rate == 0 {
		config.Rate = DefaultTTSRate
	}
	if config.Volume == 0 {
		config.Volume = DefaultTTSVolume
	}
	config.Volume = clampTTSVolume(config.Volume)
	return config
}

func parseStoredUserTTSConfig(value string) (TTSConfig, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return TTSConfig{}, false
	}
	if !strings.HasPrefix(trimmed, "{") {
		return TTSConfig{TTSConfig: appconfig.TTSConfig{Voice: trimmed}}, true
	}

	var payload storedTTSConfig
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return TTSConfig{}, false
	}

	config := TTSConfig{
		TTSConfig: appconfig.TTSConfig{
			Provider: strings.TrimSpace(payload.Provider),
			Search:   strings.TrimSpace(payload.Search),
			Voice:    pickNonEmpty(strings.TrimSpace(payload.VoiceName), strings.TrimSpace(payload.Voice)),
			Locale:   pickNonEmpty(strings.TrimSpace(payload.Language), strings.TrimSpace(payload.Locale)),
			Format:   strings.TrimSpace(payload.Format),
		},
	}
	if payload.Rate != nil {
		config.Rate = *payload.Rate
	}
	if payload.Volume != nil {
		config.Volume = *payload.Volume
	}
	if payload.Pitch != nil {
		config.Pitch = *payload.Pitch
	}
	if payload.TimeoutSeconds != nil && *payload.TimeoutSeconds > 0 {
		config.TimeoutSeconds = *payload.TimeoutSeconds
	}
	return config, true
}

func pickNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func (c *TTSClient) pickString(primary, fallback string) string {
	return pickNonEmpty(primary, fallback)
}

func (c *TTSClient) pickFloat(primary, fallback float64) float64 {
	if primary != 0 {
		return primary
	}
	return fallback
}

func clampTTSVolume(volume float64) float64 {
	if volume < 0 {
		return 0
	}
	if volume > 100 {
		return 100
	}
	return volume
}

// Name 工具名称
func (c *TTSClient) Name() string {
	return "tts_client"
}

// Description 工具描述
func (c *TTSClient) Description() string {
	return "文本转语音工具，支持 Azure / 腾讯云 TTS 直接调用"
}

// --- Helper functions ---

func sha256Hex(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func escapeSSML(text string) string {
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")
	text = strings.ReplaceAll(text, "\"", "&quot;")
	text = strings.ReplaceAll(text, "'", "&apos;")
	return text
}

func base64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// resolveTencentVoiceType maps voice names to Tencent Cloud VoiceType integers.
func resolveTencentVoiceType(voiceName string) int64 {
	lower := strings.ToLower(strings.TrimSpace(voiceName))
	// Common Mandarin voices from Tencent Cloud
	switch {
	case strings.Contains(lower, "zhiyu") || strings.Contains(lower, "智瑜"):
		return 1001
	case strings.Contains(lower, "zhiqi") || strings.Contains(lower, "智琪"):
		return 1002
	case strings.Contains(lower, "zhimei") || strings.Contains(lower, "智美"):
		return 1003
	case strings.Contains(lower, "zhilian") || strings.Contains(lower, "智联"):
		return 1004
	case strings.Contains(lower, "xiaoxiao") || strings.Contains(lower, "xiaochen"):
		return 1005
	case strings.Contains(lower, "weixiaomei") || strings.Contains(lower, "微笑美"):
		return 1006
	default:
		return 1001 // default to Zhiyu
	}
}

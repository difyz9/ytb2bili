package model

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	UserSettingKeyPreferredAIModel         = "preferred_ai_model"
	UserSettingKeyPreferredAIModelName     = "preferred_ai_model_name"
	UserSettingKeyPreferredResolution      = "preferred_resolution"
	UserSettingKeyTaskChainSettings        = "task_chain_settings"
	UserSettingKeyPlaylistSubmissionConfig = "playlist_submission_config"
	UserSettingKeyBilibiliSubmissionTID    = "bilibili_submission_tid"
	UserSettingKeyBilibiliSubmissionCopyright = "bilibili_submission_copyright"
	UserSettingKeyWatermarkPromoEnabled    = "watermark_promo_enabled"
	UserSettingKeySubtitleAudioTTSConfig   = "subtitle_audio_tts_config"
	UserSettingKeySubtitleAudioVoice       = "subtitle_audio_voice"
	UserSettingKeyTranslationModel         = "translation_model"
	UserSettingKeyMetadataModel            = "metadata_model"
	UserSettingKeyAutoUpload               = "auto_upload"
	UserSettingKeyAutoUploadInterval       = "auto_upload_interval_minutes"
	UserSettingKeyTranslationSourceLang    = "translation_source_lang"
	UserSettingKeyTranslationTargetLang    = "translation_target_lang"
	UserSettingKeyBIDDefaultLanguage       = "bid_default_language"
	UserSettingKeyBIDDefaultTone           = "bid_default_tone"
	UserSettingKeyBIDTemplateStyle         = "bid_template_style"
	UserSettingKeyAssistantSystemPrompt    = "assistant_system_prompt"
	// LLM provider settings (user-configurable)
	UserSettingKeyLLMProvider    = "llm_provider"
	UserSettingKeyLLMBaseURL     = "llm_base_url"
	UserSettingKeyLLMAPIKey      = "llm_api_key"
	UserSettingKeyLLMModel       = "llm_model"
	UserSettingKeyLLMTemperature = "llm_temperature"
	UserSettingKeyLLMMaxTokens   = "llm_max_tokens"
	// TTS provider credentials (user-configurable)
	UserSettingKeyAzureTTSSubscriptionKey = "azure_tts_subscription_key"
	UserSettingKeyAzureTTSRegion          = "azure_tts_region"
	UserSettingKeyTencentTTSSecretID      = "tencent_tts_secret_id"
	UserSettingKeyTencentTTSSecretKey     = "tencent_tts_secret_key"
	UserSettingKeyTencentTTSRegion        = "tencent_tts_region"
	DefaultAutoUploadIntervalMinutes      = 30
	DefaultBilibiliSubmissionCopyright     = 2
)

var allowedPreferredResolutions = map[string]struct{}{
	"best":  {},
	"720p":  {},
	"1080p": {},
	"1440p": {},
	"2160p": {},
}

var allowedAutoUploadIntervals = map[int]struct{}{
	15: {},
	30: {},
	60: {},
	90: {},
}

var allowedBilibiliSubmissionCopyrights = map[int]struct{}{
	1: {},
	2: {},
}

var allowedUserSettingKeys = map[string]struct{}{
	UserSettingKeyPreferredAIModel:         {},
	UserSettingKeyPreferredAIModelName:     {},
	UserSettingKeyPreferredResolution:      {},
	UserSettingKeyTaskChainSettings:        {},
	UserSettingKeyPlaylistSubmissionConfig: {},
	UserSettingKeyBilibiliSubmissionTID:    {},
	UserSettingKeyBilibiliSubmissionCopyright: {},
	UserSettingKeyWatermarkPromoEnabled:    {},
	UserSettingKeySubtitleAudioTTSConfig:   {},
	UserSettingKeySubtitleAudioVoice:       {},
	UserSettingKeyTranslationModel:         {},
	UserSettingKeyMetadataModel:            {},
	UserSettingKeyAutoUpload:               {},
	UserSettingKeyAutoUploadInterval:       {},
	UserSettingKeyTranslationSourceLang:    {},
	UserSettingKeyTranslationTargetLang:    {},
	UserSettingKeyBIDDefaultLanguage:       {},
	UserSettingKeyBIDDefaultTone:           {},
	UserSettingKeyBIDTemplateStyle:         {},
	UserSettingKeyAssistantSystemPrompt:    {},
}

type UserSettings struct {
	BaseModel
	UserID                    string     `gorm:"uniqueIndex;size:128;not null" json:"user_id"`
	AutoUploadEnabled         bool       `gorm:"default:false;index" json:"auto_upload_enabled"`
	AutoUploadIntervalMinutes int        `gorm:"default:30" json:"auto_upload_interval_minutes"`
	LastAutoUploadAt          *time.Time `json:"last_auto_upload_at,omitempty"`
	ExtraSettings             string     `gorm:"type:text" json:"-"`
}

func (UserSettings) TableName() string {
	return "tb_user_settings"
}

func IsAllowedUserSettingKey(key string) bool {
	_, ok := allowedUserSettingKeys[key]
	return ok
}

func IsAllowedAutoUploadIntervalMinutes(value int) bool {
	_, ok := allowedAutoUploadIntervals[value]
	return ok
}

func IsAllowedPreferredResolution(value string) bool {
	_, ok := allowedPreferredResolutions[strings.TrimSpace(value)]
	return ok
}

func IsAllowedBilibiliSubmissionCopyright(value int) bool {
	_, ok := allowedBilibiliSubmissionCopyrights[value]
	return ok
}

func NormalizeBilibiliSubmissionCopyright(value int) int {
	if IsAllowedBilibiliSubmissionCopyright(value) {
		return value
	}
	return DefaultBilibiliSubmissionCopyright
}

func NormalizeAutoUploadIntervalMinutes(value int) int {
	if IsAllowedAutoUploadIntervalMinutes(value) {
		return value
	}
	return DefaultAutoUploadIntervalMinutes
}

func (settings *UserSettings) ToSettingsMap() map[string]string {
	result := settings.extraSettingsMap()
	if value := ResolveSubtitleAudioTTSConfigValue(result); value != "" {
		result[UserSettingKeySubtitleAudioTTSConfig] = value
		result[UserSettingKeySubtitleAudioVoice] = value
	}
	result[UserSettingKeyAutoUpload] = boolToSettingValue(settings.AutoUploadEnabled)
	result[UserSettingKeyAutoUploadInterval] = strconv.Itoa(NormalizeAutoUploadIntervalMinutes(settings.AutoUploadIntervalMinutes))
	return result
}

func (settings *UserSettings) ApplySettingsPatch(patch map[string]string) error {
	extra := settings.extraSettingsMap()
	for key, rawValue := range patch {
		if key == UserSettingKeySubtitleAudioVoice {
			key = UserSettingKeySubtitleAudioTTSConfig
		}
		if !IsAllowedUserSettingKey(key) {
			return fmt.Errorf("unsupported setting key: %s", key)
		}

		value := strings.TrimSpace(rawValue)
		switch key {
		case UserSettingKeyAutoUpload:
			enabled, err := parseBoolSettingValue(value)
			if err != nil {
				return err
			}
			settings.AutoUploadEnabled = enabled
		case UserSettingKeyAutoUploadInterval:
			minutes, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("invalid auto upload interval: %s", value)
			}
			if !IsAllowedAutoUploadIntervalMinutes(minutes) {
				return fmt.Errorf("unsupported auto upload interval: %d", minutes)
			}
			settings.AutoUploadIntervalMinutes = minutes
		case UserSettingKeyPreferredResolution:
			if value == "" {
				delete(extra, key)
				continue
			}
			if !IsAllowedPreferredResolution(value) {
				return fmt.Errorf("unsupported preferred resolution: %s", value)
			}
			extra[key] = value
		case UserSettingKeyTaskChainSettings:
			if value == "" {
				delete(extra, key)
				continue
			}
			if !isValidTaskChainSettingsJSON(value) {
				return fmt.Errorf("invalid task chain settings payload")
			}
			extra[key] = value
		case UserSettingKeyPlaylistSubmissionConfig:
			if value == "" {
				delete(extra, key)
				continue
			}
			if !isValidPlaylistSubmissionConfigJSON(value) {
				return fmt.Errorf("invalid playlist submission config payload")
			}
			extra[key] = value
		case UserSettingKeyBilibiliSubmissionTID:
			if value == "" {
				delete(extra, key)
				continue
			}
			tid, err := strconv.Atoi(value)
			if err != nil || tid <= 0 {
				return fmt.Errorf("invalid bilibili submission tid: %s", value)
			}
			extra[key] = strconv.Itoa(tid)
		case UserSettingKeyBilibiliSubmissionCopyright:
			if value == "" {
				delete(extra, key)
				continue
			}
			copyright, err := strconv.Atoi(value)
			if err != nil || !IsAllowedBilibiliSubmissionCopyright(copyright) {
				return fmt.Errorf("invalid bilibili submission copyright: %s", value)
			}
			extra[key] = strconv.Itoa(copyright)
		case UserSettingKeyWatermarkPromoEnabled:
			enabled, err := parseBoolSettingValue(value)
			if err != nil {
				return fmt.Errorf("invalid watermark promo value: %s", value)
			}
			extra[key] = boolToSettingValue(enabled)
		default:
			extra[key] = value
		}
	}

	delete(extra, UserSettingKeyAutoUpload)
	delete(extra, UserSettingKeyAutoUploadInterval)
	if value := strings.TrimSpace(extra[UserSettingKeySubtitleAudioVoice]); value != "" {
		extra[UserSettingKeySubtitleAudioTTSConfig] = value
	}
	delete(extra, UserSettingKeySubtitleAudioVoice)

	payload, err := json.Marshal(extra)
	if err != nil {
		return fmt.Errorf("marshal user settings: %w", err)
	}

	settings.ExtraSettings = string(payload)
	settings.AutoUploadIntervalMinutes = NormalizeAutoUploadIntervalMinutes(settings.AutoUploadIntervalMinutes)
	return nil
}

func ResolveSubtitleAudioTTSConfigValue(settings map[string]string) string {
	if settings == nil {
		return ""
	}
	if value := strings.TrimSpace(settings[UserSettingKeySubtitleAudioTTSConfig]); value != "" {
		return value
	}
	return strings.TrimSpace(settings[UserSettingKeySubtitleAudioVoice])
}

func (settings *UserSettings) extraSettingsMap() map[string]string {
	result := map[string]string{}
	if strings.TrimSpace(settings.ExtraSettings) == "" {
		return result
	}

	if err := json.Unmarshal([]byte(settings.ExtraSettings), &result); err != nil {
		return map[string]string{}
	}
	return result
}

func boolToSettingValue(enabled bool) string {
	if enabled {
		return "1"
	}
	return "0"
}

func parseBoolSettingValue(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true":
		return true, nil
	case "0", "false", "":
		return false, nil
	default:
		return false, fmt.Errorf("invalid auto upload value: %s", value)
	}
}

func isValidTaskChainSettingsJSON(value string) bool {
	var payload struct {
		DownloadThumbnail       *bool `json:"download_thumbnail"`
		Transcribe              *bool `json:"transcribe"`
		TranslateSubtitles      *bool `json:"translate_subtitles"`
		SynthesizeSubtitleAudio *bool `json:"synthesize_subtitle_audio"`
	}

	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return false
	}

	return payload.DownloadThumbnail != nil ||
		payload.Transcribe != nil ||
		payload.TranslateSubtitles != nil ||
		payload.SynthesizeSubtitleAudio != nil
}

func isValidPlaylistSubmissionConfigJSON(value string) bool {
	var payload struct {
		Enabled    *bool `json:"enabled"`
		StartIndex *int  `json:"start_index"`
		MaxItems   *int  `json:"max_items"`
	}

	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return false
	}
	if payload.StartIndex != nil && *payload.StartIndex < 1 {
		return false
	}
	if payload.MaxItems != nil && (*payload.MaxItems < 1 || *payload.MaxItems > 50) {
		return false
	}

	return payload.Enabled != nil || payload.StartIndex != nil || payload.MaxItems != nil
}

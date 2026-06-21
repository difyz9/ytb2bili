package workflow

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/zolagz/ytb2bili/internal/service"
	"github.com/zolagz/ytb2bili/pkg/llm"
	storemodel "github.com/zolagz/ytb2bili/pkg/store/model"
	"github.com/zolagz/ytb2bili/pkg/tools"
	"go.uber.org/zap"
)

const preferencesAppliedContextKey contextKey = "workflow_preferences_applied"

func withPreferencesApplied(ctx context.Context) context.Context {
	return context.WithValue(ctx, preferencesAppliedContextKey, true)
}

func preferencesApplied(ctx context.Context) bool {
	if v, ok := ctx.Value(preferencesAppliedContextKey).(bool); ok {
		return v
	}
	return false
}

func applyLatestUserSettingsToVideoContext(ctx context.Context, userSettings *service.UserSettingsClient, logger *zap.Logger, vctx *VideoContext) {
	if vctx == nil {
		return
	}

	vctx.TranslationConfig = normalizeWorkflowTranslationConfig(vctx.TranslationConfig)
	vctx.SpeechSynthesisConfig = normalizeWorkflowSpeechSynthesisConfig(vctx.SpeechSynthesisConfig)
	vctx.TaskChainSettings = NormalizeTaskChainSettings(vctx.TaskChainSettings)
	vctx.PreferredResolution = defaultWorkflowPreferredResolution(vctx.PreferredResolution)

	userID := strings.TrimSpace(vctx.UserID)
	if userID == "" {
		userID = strings.TrimSpace(GetUserID(ctx))
		if userID != "" {
			vctx.UserID = userID
		}
	}

	if userSettings == nil || !userSettings.IsEnabled() || userID == "" {
		return
	}

	settings, err := userSettings.GetSettings(ctx, userID)
	if err != nil {
		if logger != nil {
			logger.Warn("Failed to refresh workflow preferences from database",
				zap.String("user_id", userID),
				zap.Error(err))
		}
		return
	}

	if resolution := normalizeWorkflowPreferredResolution(settings[storemodel.UserSettingKeyPreferredResolution]); resolution != "" {
		vctx.PreferredResolution = resolution
	}
	if sourceLang := strings.TrimSpace(settings[storemodel.UserSettingKeyTranslationSourceLang]); sourceLang != "" {
		vctx.TranslationConfig.SourceLanguage = sourceLang
	}
	if targetLang := strings.TrimSpace(settings[storemodel.UserSettingKeyTranslationTargetLang]); targetLang != "" {
		vctx.TranslationConfig.TargetLanguage = targetLang
	}
	if modelName := resolvePreferredWorkflowModel(settings, storemodel.UserSettingKeyTranslationModel); modelName != "" {
		vctx.TranslationConfig.ModelName = modelName
	}
	if taskChainSettings := parseWorkflowTaskChainSettings(settings[storemodel.UserSettingKeyTaskChainSettings]); taskChainSettings != nil {
		vctx.TaskChainSettings = taskChainSettings
	}
	userExplicitTTS := false
	if speechConfig := parseWorkflowSpeechSynthesisConfig(storemodel.ResolveSubtitleAudioTTSConfigValue(settings)); speechConfig != nil {
		vctx.SpeechSynthesisConfig = speechConfig
		userExplicitTTS = true
	}

	if !userExplicitTTS {
		syncTTSLanguageFromTranslationTarget(vctx, logger)
	}

	vctx.TranslationConfig = normalizeWorkflowTranslationConfig(vctx.TranslationConfig)
	vctx.SpeechSynthesisConfig = normalizeWorkflowSpeechSynthesisConfig(vctx.SpeechSynthesisConfig)
	vctx.TaskChainSettings = NormalizeTaskChainSettings(vctx.TaskChainSettings)
	vctx.PreferredResolution = defaultWorkflowPreferredResolution(vctx.PreferredResolution)

	if logger != nil {
		logger.Info("Refreshed workflow preferences from database",
			zap.String("user_id", userID),
			zap.String("preferred_resolution", vctx.PreferredResolution),
			zap.String("translation_model", vctx.TranslationConfig.ModelName),
			zap.String("translation_source_lang", vctx.TranslationConfig.SourceLanguage),
			zap.String("translation_target_lang", vctx.TranslationConfig.TargetLanguage),
			zap.String("subtitle_voice", vctx.SpeechSynthesisConfig.VoiceName))
	}
}

func syncTTSLanguageFromTranslationTarget(vctx *VideoContext, logger *zap.Logger) {
	targetLang := strings.TrimSpace(vctx.TranslationConfig.TargetLanguage)
	if targetLang == "" {
		return
	}
	defaults, ok := translationLangToTTSDefaults[targetLang]
	if !ok {
		return
	}
	vctx.SpeechSynthesisConfig.Language = defaults.Locale
	vctx.SpeechSynthesisConfig.VoiceName = defaults.Voice
	if logger != nil {
		logger.Info("Auto-synced TTS language from translation target",
			zap.String("translation_target", targetLang),
			zap.String("tts_locale", defaults.Locale),
			zap.String("tts_voice", defaults.Voice))
	}
}

func resolvePreferredWorkflowModel(settings map[string]string, explicitKeys ...string) string {
	for _, key := range explicitKeys {
		if value := strings.TrimSpace(settings[key]); value != "" {
			return value
		}
	}
	if value := strings.TrimSpace(settings[storemodel.UserSettingKeyPreferredAIModel]); value != "" {
		return value
	}
	return ""
}

func normalizeWorkflowTranslationConfig(config *TranslationConfig) *TranslationConfig {
	if config == nil {
		config = &TranslationConfig{}
	}
	if strings.TrimSpace(config.SourceLanguage) == "" {
		config.SourceLanguage = "en"
	}
	if strings.TrimSpace(config.TargetLanguage) == "" {
		config.TargetLanguage = "zh-Hans"
	}
	if strings.TrimSpace(config.ModelName) == "" {
		config.ModelName = llm.DefaultTranslationModel
	}
	return config
}

func normalizeWorkflowSpeechSynthesisConfig(config *SpeechSynthesisConfig) *SpeechSynthesisConfig {
	normalized := &SpeechSynthesisConfig{
		Language:  tools.DefaultTTSLocale,
		VoiceName: tools.DefaultTTSVoice,
		Format:    "mp3",
	}
	if config == nil {
		return normalized
	}
	if value := strings.TrimSpace(config.Language); value != "" {
		normalized.Language = value
	}
	if value := strings.TrimSpace(config.VoiceName); value != "" {
		normalized.VoiceName = value
	}
	if value := strings.TrimSpace(config.Format); value != "" {
		normalized.Format = value
	}
	if value := strings.TrimSpace(config.Provider); value != "" {
		normalized.Provider = value
	}
	if value := strings.TrimSpace(config.Search); value != "" {
		normalized.Search = value
	}
	if config.Rate != 0 {
		normalized.Rate = config.Rate
	}
	if config.Volume != 0 {
		normalized.Volume = config.Volume
	}
	if config.Pitch != 0 {
		normalized.Pitch = config.Pitch
	}
	return normalized
}

func ParseSpeechSynthesisConfigValue(value string) *SpeechSynthesisConfig {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}

	type storedSpeechConfig struct {
		Language  string   `json:"language"`
		Locale    string   `json:"locale"`
		VoiceName string   `json:"voice_name"`
		Voice     string   `json:"voice"`
		Format    string   `json:"format"`
		Provider  string   `json:"provider"`
		Search    string   `json:"search"`
		Rate      *float64 `json:"rate"`
		Volume    *float64 `json:"volume"`
		Pitch     *float64 `json:"pitch"`
	}

	if strings.HasPrefix(trimmed, "{") {
		var payload storedSpeechConfig
		if err := json.Unmarshal([]byte(trimmed), &payload); err == nil {
			config := &SpeechSynthesisConfig{
				Language:  pickNonEmpty(payload.Language, payload.Locale),
				VoiceName: pickNonEmpty(payload.VoiceName, payload.Voice),
				Format:    strings.TrimSpace(payload.Format),
				Provider:  strings.TrimSpace(payload.Provider),
				Search:    strings.TrimSpace(payload.Search),
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
			return normalizeWorkflowSpeechSynthesisConfig(config)
		}
	}

	return normalizeWorkflowSpeechSynthesisConfig(&SpeechSynthesisConfig{
		Language:  "zh-CN",
		VoiceName: trimmed,
		Format:    "mp3",
	})
}

func parseWorkflowSpeechSynthesisConfig(value string) *SpeechSynthesisConfig {
	return ParseSpeechSynthesisConfigValue(value)
}

// ttsLangDefault holds the default TTS locale and voice for a translation target language.
type ttsLangDefault struct {
	Locale string
	Voice  string
}

// translationLangToTTSDefaults maps common translation target language codes
// to sensible TTS locale and voice defaults so that changing the translation
// target language automatically produces speech in the matching language when
// the user has not explicitly configured a TTS voice.
var translationLangToTTSDefaults = map[string]ttsLangDefault{
	"zh-Hans": {Locale: "zh-CN", Voice: "zh-CN-XiaoxiaoNeural"},
	"zh-Hant": {Locale: "zh-TW", Voice: "zh-TW-HsiaoChenNeural"},
	"zh-CN":   {Locale: "zh-CN", Voice: "zh-CN-XiaoxiaoNeural"},
	"zh-TW":   {Locale: "zh-TW", Voice: "zh-TW-HsiaoChenNeural"},
	"zh":      {Locale: "zh-CN", Voice: "zh-CN-XiaoxiaoNeural"},
	"en":      {Locale: "en-US", Voice: "en-US-JennyNeural"},
	"en-US":   {Locale: "en-US", Voice: "en-US-JennyNeural"},
	"en-GB":   {Locale: "en-GB", Voice: "en-GB-SoniaNeural"},
	"ja":      {Locale: "ja-JP", Voice: "ja-JP-NanamiNeural"},
	"ko":      {Locale: "ko-KR", Voice: "ko-KR-SunHiNeural"},
	"fr":      {Locale: "fr-FR", Voice: "fr-FR-DeniseNeural"},
	"de":      {Locale: "de-DE", Voice: "de-DE-KatjaNeural"},
	"es":      {Locale: "es-ES", Voice: "es-ES-ElviraNeural"},
	"pt":      {Locale: "pt-BR", Voice: "pt-BR-FranciscaNeural"},
	"ru":      {Locale: "ru-RU", Voice: "ru-RU-SvetlanaNeural"},
	"ar":      {Locale: "ar-SA", Voice: "ar-SA-ZariyahNeural"},
	"hi":      {Locale: "hi-IN", Voice: "hi-IN-SwaraNeural"},
	"th":      {Locale: "th-TH", Voice: "th-TH-PremwadeeNeural"},
	"vi":      {Locale: "vi-VN", Voice: "vi-VN-HoaiMyNeural"},
	"id":      {Locale: "id-ID", Voice: "id-ID-GadisNeural"},
	"ms":      {Locale: "ms-MY", Voice: "ms-MY-YasminNeural"},
	"it":      {Locale: "it-IT", Voice: "it-IT-ElsaNeural"},
	"tr":      {Locale: "tr-TR", Voice: "tr-TR-EmelNeural"},
	"pl":      {Locale: "pl-PL", Voice: "pl-PL-AgnieszkaNeural"},
	"nl":      {Locale: "nl-NL", Voice: "nl-NL-ColetteNeural"},
	"uk":      {Locale: "uk-UA", Voice: "uk-UA-PolinaNeural"},
}

func pickNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func parseWorkflowTaskChainSettings(value string) *TaskChainSettings {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}

	var settings TaskChainSettings
	if err := json.Unmarshal([]byte(trimmed), &settings); err != nil {
		return nil
	}

	return NormalizeTaskChainSettings(&settings)
}

func normalizeWorkflowPreferredResolution(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return ""
	}
	if storemodel.IsAllowedPreferredResolution(normalized) {
		return normalized
	}
	return ""
}

func defaultWorkflowPreferredResolution(value string) string {
	if normalized := normalizeWorkflowPreferredResolution(value); normalized != "" {
		return normalized
	}
	return "best"
}

package handler

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/zolagz/ytb2bili/internal/config"
	"github.com/zolagz/ytb2bili/internal/service"
	"github.com/zolagz/ytb2bili/internal/workflow"
	"github.com/zolagz/ytb2bili/pkg/llm"
	"github.com/zolagz/ytb2bili/pkg/utils"
	storemodel "github.com/zolagz/ytb2bili/pkg/store/model"
)

func ResolveVideoTranslationConfig(
	ctx context.Context,
	cfg *config.AppConfig,
	userSettings *service.UserSettingsClient,
	userID string,
	overrides *workflow.TranslationConfig,
) *workflow.TranslationConfig {
	sourceLang := cfg.Workflow.LLMTranslationSourceLang
	if sourceLang == "" {
		sourceLang = "en"
	}
	targetLang := cfg.Workflow.LLMTranslationTargetLang
	if targetLang == "" {
		targetLang = "zh-Hans"
	}

	tc := &workflow.TranslationConfig{
		SourceLanguage: sourceLang,
		TargetLanguage: targetLang,
		ModelName:      llm.DefaultTranslationModel,
	}

	if overrides != nil {
		if overrides.SourceLanguage != "" {
			tc.SourceLanguage = overrides.SourceLanguage
		}
		if overrides.TargetLanguage != "" {
			tc.TargetLanguage = overrides.TargetLanguage
		}
		if overrides.ModelName != "" {
			tc.ModelName = overrides.ModelName
		}
	}

	if userSettings != nil && userSettings.IsEnabled() && strings.TrimSpace(userID) != "" {
		settings, err := userSettings.GetSettings(ctx, userID)
		if err == nil {
			if v := strings.TrimSpace(settings[storemodel.UserSettingKeyTranslationSourceLang]); v != "" {
				tc.SourceLanguage = v
			}
			if v := strings.TrimSpace(settings[storemodel.UserSettingKeyTranslationTargetLang]); v != "" {
				tc.TargetLanguage = v
			}
			if v := strings.TrimSpace(settings[storemodel.UserSettingKeyTranslationModel]); v != "" {
				tc.ModelName = v
			}
		}
	}
	return tc
}

type videoProcessingPreferences struct {
	PreferredResolution   string
	TaskChainSettings     *workflow.TaskChainSettings
	SpeechSynthesisConfig *workflow.SpeechSynthesisConfig
}

func ResolveVideoProcessingPreferences(
	ctx context.Context,
	userSettings *service.UserSettingsClient,
	userID string,
	preferredResolution string,
	taskChainSettings *workflow.TaskChainSettings,
	speechSynthesisConfig *workflow.SpeechSynthesisConfig,
) videoProcessingPreferences {
	resolution := utils.NormalizeResolution(preferredResolution)
	chain := workflow.NormalizeTaskChainSettings(taskChainSettings)
	speech := speechConfigOrDefault(speechSynthesisConfig)
	return videoProcessingPreferences{
		PreferredResolution:   resolution,
		TaskChainSettings:     chain,
		SpeechSynthesisConfig: speech,
	}
}

func ParseStoredTaskChainSettings(data string) *workflow.TaskChainSettings {
	if strings.TrimSpace(data) == "" {
		return workflow.DefaultTaskChainSettings()
	}
	var s workflow.TaskChainSettings
	if err := json.Unmarshal([]byte(data), &s); err != nil {
		return workflow.DefaultTaskChainSettings()
	}
	return workflow.NormalizeTaskChainSettings(&s)
}

func BuildSpeechSynthesisConfigFromVoiceName(voiceName string) *workflow.SpeechSynthesisConfig {
	v := strings.TrimSpace(voiceName)
	if v == "" {
		v = "zh-CN-XiaoxiaoNeural"
	}
	return &workflow.SpeechSynthesisConfig{Language: "zh-CN", VoiceName: v, Format: "mp3"}
}

func speechConfigOrDefault(cfg *workflow.SpeechSynthesisConfig) *workflow.SpeechSynthesisConfig {
	if cfg == nil {
		return BuildSpeechSynthesisConfigFromVoiceName("")
	}
	n := *cfg
	if n.Language == "" {
		n.Language = "zh-CN"
	}
	if n.VoiceName == "" {
		n.VoiceName = "zh-CN-XiaoxiaoNeural"
	}
	if n.Format == "" {
		n.Format = "mp3"
	}
	return &n
}

func serializeTaskChainSettings(settings *workflow.TaskChainSettings) string {
	if settings == nil {
		return ""
	}
	data, _ := json.Marshal(settings)
	return string(data)
}

func extractYouTubeVideoID(rawURL string) string {
	patterns := []string{
		`(?:v=|youtu\.be/|shorts/|embed/)([A-Za-z0-9_-]{11})`,
	}
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(rawURL); len(matches) > 1 {
			return matches[1]
		}
	}
	return ""
}

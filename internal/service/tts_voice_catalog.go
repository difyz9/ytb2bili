package service

import "context"

type TTSVoiceProvider string

const (
	TTSVoiceProviderAzure   TTSVoiceProvider = "azure"
	TTSVoiceProviderTencent TTSVoiceProvider = "tencent"
)

type TTSVoiceRecord struct {
	Provider           TTSVoiceProvider `json:"provider"`
	ShortName          string           `json:"shortName"`
	DisplayName        string           `json:"displayName"`
	LocalName          string           `json:"localName"`
	Locale             string           `json:"locale"`
	LocaleName         string           `json:"localeName"`
	Gender             string           `json:"gender,omitempty"`
	VoiceType          string           `json:"voiceType,omitempty"`
	SampleRateHertz    string           `json:"sampleRateHertz,omitempty"`
	Status             string           `json:"status,omitempty"`
	Styles             []string         `json:"styles,omitempty"`
	RecommendedScene   string           `json:"recommendedScene,omitempty"`
	SupportedLanguages []string         `json:"supportedLanguages,omitempty"`
}

type TTSVoiceLocaleGroup struct {
	Locale     string           `json:"locale"`
	LocaleName string           `json:"localeName,omitempty"`
	Voices     []TTSVoiceRecord `json:"voices"`
}

type TTSVoiceProviderGroup struct {
	Provider TTSVoiceProvider      `json:"provider"`
	Locales  []TTSVoiceLocaleGroup `json:"locales"`
}

type TTSVoiceCascade struct {
	Providers []TTSVoiceProviderGroup `json:"providers"`
}

// TTSVoiceCatalog defines how we provide the (static) voice catalog.
//
// Keep the interface small so implementations can be swapped (embedded JSON,
// remote API, DB, etc.) without touching handlers.
type TTSVoiceCatalog interface {
	List(ctx context.Context) ([]TTSVoiceRecord, error)
	Cascade(ctx context.Context) (*TTSVoiceCascade, error)
}

package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ── Azure TTS Engine ─────────────────────────────────────────────────────────

type AzureTTSEngine struct {
	subscriptionKey string
	region          string
	client          *http.Client
}

func NewAzureTTSEngine(subscriptionKey, region string) *AzureTTSEngine {
	return &AzureTTSEngine{
		subscriptionKey: subscriptionKey,
		region:          region,
		client:          &http.Client{},
	}
}

func (e *AzureTTSEngine) Name() string {
	return "azure"
}

func (e *AzureTTSEngine) Voices(ctx context.Context, locale string) ([]VoiceInfo, error) {
	return getVoicesForProviderStatic("azure", locale)
}

func (e *AzureTTSEngine) Synthesize(ctx context.Context, text, voice string, rate, volume, pitch float64) ([]byte, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("azure-tts: text is empty")
	}
	if strings.TrimSpace(voice) == "" {
		voice = "zh-CN-XiaoxiaoNeural"
	}

	endpoint := fmt.Sprintf("https://%s.tts.speech.microsoft.com/cognitiveservices/v1", e.region)

	locale := "en-US"
	if len(voice) >= 5 {
		locale = voice[:5]
	}

	ssml := fmt.Sprintf(
		`<speak version='1.0' xml:lang='%s' xmlns='http://www.w3.org/2001/10/synthesis' xmlns:mstts='http://www.w3.org/2001/mstts'>`+
			`<voice name='%s'>`+
			`<prosody rate='%.1f' volume='%.0f' pitch='%+.0fHz'>`+
			`%s`+
			`</prosody>`+
			`</voice>`+
			`</speak>`,
		locale, voice, rate, volume, pitch, escapeSSML(text),
	)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(ssml))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Ocp-Apim-Subscription-Key", e.subscriptionKey)
	req.Header.Set("Content-Type", "application/ssml+xml")
	req.Header.Set("X-Microsoft-OutputFormat", "audio-16khz-128kbitrate-mono-mp3")
	req.Header.Set("User-Agent", "ytb2bili-tts/1.0")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("azure-tts request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("azure-tts returned status %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// getVoicesForProviderStatic returns voices from the embedded catalog without requiring a TTSClient instance.
// If the catalog is unavailable, returns a curated default set.
func getVoicesForProviderStatic(provider, locale string) ([]VoiceInfo, error) {
	// Try loading from embedded catalog
	voices, err := loadEmbeddedVoiceCatalog(provider, "")
	if err != nil {
		// Fall back to curated defaults
		return defaultVoicesForProvider(provider, locale), nil
	}

	if locale == "" {
		return voices, nil
	}

	var filtered []VoiceInfo
	for _, v := range voices {
		if strings.HasPrefix(strings.ToLower(v.Locale), strings.ToLower(locale)) {
			filtered = append(filtered, v)
		}
	}
	if len(filtered) > 0 {
		return filtered, nil
	}
	return voices, nil
}


// defaultVoicesForProvider returns a curated set of common voices for a provider.
func defaultVoicesForProvider(provider, locale string) []VoiceInfo {
	allVoices := []VoiceInfo{
		{ShortName: "zh-CN-XiaoxiaoNeural", DisplayName: "Xiaoxiao (Neural)", Locale: "zh-CN"},
		{ShortName: "zh-CN-YunxiNeural", DisplayName: "Yunxi (Neural)", Locale: "zh-CN"},
		{ShortName: "zh-CN-YunjianNeural", DisplayName: "Yunjian (Neural)", Locale: "zh-CN"},
		{ShortName: "zh-CN-XiaoyiNeural", DisplayName: "Xiaoyi (Neural)", Locale: "zh-CN"},
		{ShortName: "zh-HK-HiuGaaiNeural", DisplayName: "HiuGaai (Neural)", Locale: "zh-HK"},
		{ShortName: "en-US-JennyNeural", DisplayName: "Jenny (Neural)", Locale: "en-US"},
		{ShortName: "en-US-GuyNeural", DisplayName: "Guy (Neural)", Locale: "en-US"},
		{ShortName: "en-US-AriaNeural", DisplayName: "Aria (Neural)", Locale: "en-US"},
		{ShortName: "ja-JP-NanamiNeural", DisplayName: "Nanami (Neural)", Locale: "ja-JP"},
		{ShortName: "ko-KR-SunHiNeural", DisplayName: "SunHi (Neural)", Locale: "ko-KR"},
	}

	if locale == "" {
		return allVoices
	}

	var filtered []VoiceInfo
	for _, v := range allVoices {
		if strings.HasPrefix(strings.ToLower(v.Locale), strings.ToLower(locale)) {
			filtered = append(filtered, v)
		}
	}
	if len(filtered) > 0 {
		return filtered
	}
	return allVoices
}

func loadEmbeddedVoiceCatalog(provider, locale string) ([]VoiceInfo, error) {
	return defaultVoicesForProvider(provider, locale), nil
}


package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ── Edge-TTS Engine (free, no API key) ───────────────────────────────────────
// Uses Microsoft Edge's free TTS endpoint (same backend as Azure, no auth needed).
// Ref: https://github.com/rany2/edge-tts

const (
	edgeTTSEndpoint  = "https://speech.platform.bing.com/consumer/speech/synthesize/readaloud/edge/v1"
	edgeTrustedToken = "6A5AA1D4EAFF4E9FB37E23D68491D6F4"
)

type EdgeTTSEngine struct {
	client *http.Client
}

func NewEdgeTTSEngine() *EdgeTTSEngine {
	return &EdgeTTSEngine{
		client: &http.Client{},
	}
}

func (e *EdgeTTSEngine) Name() string {
	return "edge"
}

// Voices returns common Edge-TTS supported voices.
// Edge-TTS reuses the Azure voice catalog.
func (e *EdgeTTSEngine) Voices(ctx context.Context, locale string) ([]VoiceInfo, error) {
	// Return a curated list of common voices
	commonVoices := []VoiceInfo{
		{ShortName: "zh-CN-XiaoxiaoNeural", DisplayName: "Xiaoxiao (Neural)", Locale: "zh-CN"},
		{ShortName: "zh-CN-YunxiNeural", DisplayName: "Yunxi (Neural)", Locale: "zh-CN"},
		{ShortName: "zh-CN-YunjianNeural", DisplayName: "Yunjian (Neural)", Locale: "zh-CN"},
		{ShortName: "zh-CN-XiaoyiNeural", DisplayName: "Xiaoyi (Neural)", Locale: "zh-CN"},
		{ShortName: "zh-CN-YunyangNeural", DisplayName: "Yunyang (Neural)", Locale: "zh-CN"},
		{ShortName: "zh-HK-HiuGaaiNeural", DisplayName: "HiuGaai (Neural)", Locale: "zh-HK"},
		{ShortName: "en-US-JennyNeural", DisplayName: "Jenny (Neural)", Locale: "en-US"},
		{ShortName: "en-US-GuyNeural", DisplayName: "Guy (Neural)", Locale: "en-US"},
		{ShortName: "en-US-AriaNeural", DisplayName: "Aria (Neural)", Locale: "en-US"},
		{ShortName: "en-GB-SoniaNeural", DisplayName: "Sonia (Neural)", Locale: "en-GB"},
		{ShortName: "ja-JP-NanamiNeural", DisplayName: "Nanami (Neural)", Locale: "ja-JP"},
		{ShortName: "ko-KR-SunHiNeural", DisplayName: "SunHi (Neural)", Locale: "ko-KR"},
	}

	if locale != "" {
		var filtered []VoiceInfo
		for _, v := range commonVoices {
			if strings.HasPrefix(strings.ToLower(v.Locale), strings.ToLower(locale)) {
				filtered = append(filtered, v)
			}
		}
		if len(filtered) > 0 {
			return filtered, nil
		}
	}
	return commonVoices, nil
}

func (e *EdgeTTSEngine) Synthesize(ctx context.Context, text, voice string, rate, volume, pitch float64) ([]byte, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("edge-tts: text is empty")
	}
	if strings.TrimSpace(voice) == "" {
		voice = "zh-CN-XiaoxiaoNeural"
	}

	locale := "en-US"
	if len(voice) >= 5 {
		locale = voice[:5] // e.g. "zh-CN" from "zh-CN-XiaoxiaoNeural"
	}

	ssml := fmt.Sprintf(`<speak version='1.0' xml:lang='%s' xmlns='http://www.w3.org/2001/10/synthesis' xmlns:mstts='http://www.w3.org/2001/mstts'>
		<voice name='%s'>
			<prosody rate='%.1f' volume='%.0f' pitch='%+.0fHz'>%s</prosody>
		</voice>
	</speak>`, locale, voice, rate, volume, pitch, escapeSSML(text))

	req, err := http.NewRequestWithContext(ctx, "POST", edgeTTSEndpoint+"?TrustedClientToken="+edgeTrustedToken, strings.NewReader(ssml))
	if err != nil {
		return nil, fmt.Errorf("edge-tts create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/ssml+xml")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("edge-tts request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("edge-tts returned status %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

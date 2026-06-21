package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ── OpenAI TTS Engine ────────────────────────────────────────────────────────
// Uses OpenAI's Audio Speech API: POST /v1/audio/speech
// Only supports the "tts-1" and "tts-1-hd" models.
// The API key can be from [chat] config or a dedicated [openai_tts] config.

const (
	openAITTSEndpoint = "https://api.openai.com/v1/audio/speech"
	openAITTSPrefix   = "openai-tts"
)

type OpenAITTSEngine struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// NewOpenAITTSEngine creates an OpenAI TTS engine.
// apiKey: required, OpenAI API key.
// baseURL: optional, for custom endpoints (e.g., proxy). Defaults to api.openai.com.
// model: optional, defaults to "tts-1".
func NewOpenAITTSEngine(apiKey, baseURL, model string) *OpenAITTSEngine {
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	if model == "" {
		model = "tts-1"
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &OpenAITTSEngine{
		apiKey:  strings.TrimSpace(apiKey),
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{},
	}
}

func (e *OpenAITTSEngine) Name() string {
	return "openai"
}

// OpenAI TTS voices
var openAIVoices = []VoiceInfo{
	{ShortName: "alloy", DisplayName: "Alloy", Locale: "en-US"},
	{ShortName: "echo", DisplayName: "Echo", Locale: "en-US"},
	{ShortName: "fable", DisplayName: "Fable", Locale: "en-US"},
	{ShortName: "onyx", DisplayName: "Onyx", Locale: "en-US"},
	{ShortName: "nova", DisplayName: "Nova", Locale: "en-US"},
	{ShortName: "shimmer", DisplayName: "Shimmer", Locale: "en-US"},
}

func (e *OpenAITTSEngine) Voices(ctx context.Context, locale string) ([]VoiceInfo, error) {
	return openAIVoices, nil
}

type openAITTSSpeechReq struct {
	Model          string `json:"model"`
	Input          string `json:"input"`
	Voice          string `json:"voice"`
	ResponseFormat string `json:"response_format"`
	Speed          float64 `json:"speed"`
}

func (e *OpenAITTSEngine) Synthesize(ctx context.Context, text, voice string, rate, volume, pitch float64) ([]byte, error) {
	if strings.TrimSpace(e.apiKey) == "" {
		return nil, fmt.Errorf("openai-tts: API key not configured")
	}
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("openai-tts: text is empty")
	}
	if strings.TrimSpace(voice) == "" {
		voice = "alloy"
	}

	// Map rate to OpenAI speed (0.25 - 4.0, default 1.0)
	speed := rate
	if speed <= 0 {
		speed = 1.0
	}
	// Clamp to OpenAI's supported range
	if speed < 0.25 {
		speed = 0.25
	}
	if speed > 4.0 {
		speed = 4.0
	}

	body := openAITTSSpeechReq{
		Model:          e.model,
		Input:          text,
		Voice:          voice,
		ResponseFormat: "mp3",
		Speed:          speed,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("openai-tts marshal: %w", err)
	}

	endpoint := strings.TrimRight(e.baseURL, "/") + "/v1/audio/speech"
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("openai-tts create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai-tts request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai-tts returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return io.ReadAll(resp.Body)
}

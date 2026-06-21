package tools

import (
	"testing"

	appconfig "github.com/zolagz/ytb2bili/internal/config"
	"go.uber.org/zap"
)

func TestBuildRuntimeTTSConfigUsesAzureSection(t *testing.T) {
	cfg := &appconfig.AppConfig{
		LLM: appconfig.LLMConfig{
			BaseURL: "https://api.deepseek.com",
			APIKey:  "sk-test",
		},
		TTS: appconfig.TTSConfig{
			Provider: "auto",
			Voice:    "zh-CN-XiaoxiaoNeural",
			Locale:   "zh-CN",
		},
		AzureTTS: appconfig.AzureTTSConfig{
			SubscriptionKey: "azure-subscription",
			Region:          "eastasia",
		},
	}

	runtimeCfg := buildRuntimeTTSConfig(cfg)
	if runtimeCfg.AzureSubscriptionKey != "azure-subscription" {
		t.Fatalf("expected azure subscription key to be loaded, got %q", runtimeCfg.AzureSubscriptionKey)
	}
	if runtimeCfg.AzureRegion != "eastasia" {
		t.Fatalf("expected azure region to be loaded, got %q", runtimeCfg.AzureRegion)
	}
}

func TestParseStoredUserTTSConfig_LegacyVoiceValue(t *testing.T) {
	config, ok := parseStoredUserTTSConfig("zh-CN-YunxiNeural")
	if !ok {
		t.Fatal("expected legacy voice value to be parsed")
	}
	if config.Voice != "zh-CN-YunxiNeural" {
		t.Fatalf("expected voice to be parsed, got %q", config.Voice)
	}
	if config.Provider != "" {
		t.Fatalf("expected provider to remain empty, got %q", config.Provider)
	}
}

func TestParseStoredUserTTSConfig_JSONValue(t *testing.T) {
	config, ok := parseStoredUserTTSConfig(`{"provider":"tencent","voice_name":"zh-CN-YunxiNeural","language":"zh-CN","format":"mp3","search":"Yunxi","rate":1.25,"volume":88,"pitch":3,"timeout_seconds":45}`)
	if !ok {
		t.Fatal("expected json config to be parsed")
	}
	if config.Provider != "tencent" {
		t.Fatalf("expected provider from json, got %q", config.Provider)
	}
	if config.Voice != "zh-CN-YunxiNeural" {
		t.Fatalf("expected voice from json, got %q", config.Voice)
	}
	if config.Search != "Yunxi" {
		t.Fatalf("expected search from json, got %q", config.Search)
	}
	if config.Format != "mp3" {
		t.Fatalf("expected format from json, got %q", config.Format)
	}
	if config.Rate != 1.25 {
		t.Fatalf("expected rate from json, got %v", config.Rate)
	}
	if config.Volume != 88 {
		t.Fatalf("expected volume from json, got %v", config.Volume)
	}
	if config.Pitch != 3 {
		t.Fatalf("expected pitch from json, got %v", config.Pitch)
	}
	if config.TimeoutSeconds != 45 {
		t.Fatalf("expected timeout from json, got %v", config.TimeoutSeconds)
	}
}

func TestMergeConfig_UsesUserOverridesAndKeepsGlobalCredentials(t *testing.T) {
	client := newTTSClientWithConfig(TTSConfig{
		BaseURL:   "https://open.ytb2bili.com",
		APIKey:    "sk-test",
		ProjectID: "project-default",
		TTSConfig: appconfig.TTSConfig{
			Provider: "auto",
			Voice:    "zh-CN-XiaoxiaoNeural",
			Locale:   "zh-CN",
			Format:   "audio-24khz-96kbitrate-mono-mp3",
			Rate:     1,
			Volume:   100,
		},
	}, nil, zap.NewNop())

	config := client.mergeConfig(client.config, TTSConfig{
		TTSConfig: appconfig.TTSConfig{
			Provider: "tencent",
			Voice:    "zh-CN-YunxiNeural",
			Search:   "Yunxi",
			Format:   "mp3",
			Rate:     1.25,
			Volume:   88,
			Pitch:    3,
		},
	})
	if config.BaseURL != "https://open.ytb2bili.com" {
		t.Fatalf("expected baseURL to stay global, got %q", config.BaseURL)
	}
	if config.APIKey != "sk-test" {
		t.Fatalf("expected apiKey to stay global, got %q", config.APIKey)
	}
	if config.Provider != "tencent" {
		t.Fatalf("expected provider override, got %q", config.Provider)
	}
	if config.Voice != "zh-CN-YunxiNeural" {
		t.Fatalf("expected voice override, got %q", config.Voice)
	}
	if config.Search != "Yunxi" {
		t.Fatalf("expected search override, got %q", config.Search)
	}
	if config.Format != "mp3" {
		t.Fatalf("expected format override, got %q", config.Format)
	}
	if config.Rate != 1.25 {
		t.Fatalf("expected rate override, got %v", config.Rate)
	}
	if config.Volume != 88 {
		t.Fatalf("expected volume override, got %v", config.Volume)
	}
	if config.Pitch != 3 {
		t.Fatalf("expected pitch override, got %v", config.Pitch)
	}
}

func TestNormalizeTTSConfig_ClampsVolumeIntoSupportedRange(t *testing.T) {
	config := normalizeTTSConfig(TTSConfig{
		TTSConfig: appconfig.TTSConfig{
			Volume: 250,
		},
	})
	if config.Volume != 100 {
		t.Fatalf("expected volume to be clamped to 100, got %v", config.Volume)
	}

	config = normalizeTTSConfig(TTSConfig{
		TTSConfig: appconfig.TTSConfig{
			Volume: -15,
		},
	})
	if config.Volume != 0 {
		t.Fatalf("expected negative volume to be clamped to 0, got %v", config.Volume)
	}
}

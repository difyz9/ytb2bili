package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/zolagz/ytb2bili/internal/config"
	"github.com/zolagz/ytb2bili/pkg/llm"
	"go.uber.org/zap"
)

func TestDeepSeekBatchTranslatorFromConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DeepSeek integration test in short mode")
	}

	configPath := locateDeepSeekTestConfig(t)
	appCfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config %q: %v", configPath, err)
	}

	provider := appCfg.ResolveTranslationProvider()
	if provider == nil {
		t.Fatal("translation provider config is nil")
	}
	if provider.Provider != "deepseek" {
		t.Fatalf("expected deepseek provider, got %q", provider.Provider)
	}
	if provider.BaseURL != "https://api.deepseek.com" {
		t.Fatalf("expected DeepSeek base URL, got %q", provider.BaseURL)
	}
	if provider.Model != "deepseek-v4-flash" {
		t.Fatalf("expected DeepSeek model deepseek-v4-flash, got %q", provider.Model)
	}
	if strings.TrimSpace(provider.APIKey) == "" {
		t.Fatal("DeepSeek API key is empty in config")
	}

	client, err := llm.NewClientFromConfig(provider.ToLLMConfig(), zap.NewNop())
	if err != nil {
		t.Fatalf("create DeepSeek client: %v", err)
	}

	translator := NewBatchTranslator(client, BatchTranslatorConfig{
		SourceLang:  "en",
		TargetLang:  "zh-Hans",
		BatchSize:   2,
		MaxWorkers:  1,
		RetryCount:  1,
		ContextSize: 1,
	}, zap.NewNop())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	input := []string{
		"Hello everyone, welcome back to the channel.",
		"Today we will show you how to translate subtitles with DeepSeek.",
		"Please make sure the translated subtitles stay natural and concise.",
	}

	result, err := translator.TranslateTexts(ctx, input)
	if err != nil {
		t.Fatalf("DeepSeek subtitle translation failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected translation result, got nil")
	}
	if result.SkippedTranslation {
		t.Fatal("expected translation to run, but it was skipped")
	}
	if len(result.TranslatedTexts) != len(input) {
		t.Fatalf("expected %d translated lines, got %d", len(input), len(result.TranslatedTexts))
	}

	hasHan := false
	for i, translated := range result.TranslatedTexts {
		trimmed := strings.TrimSpace(translated)
		if trimmed == "" {
			t.Fatalf("translated line %d is empty", i)
		}
		if trimmed == input[i] {
			t.Fatalf("translated line %d was not translated: %q", i, trimmed)
		}
		if containsHan(trimmed) {
			hasHan = true
		}
	}
	if !hasHan {
		t.Fatalf("expected at least one translated line to contain Chinese characters, got %#v", result.TranslatedTexts)
	}
	if result.Duration <= 0 {
		t.Fatalf("expected positive translation duration, got %v", result.Duration)
	}
	if t.Failed() {
		return
	}
	t.Logf("DeepSeek subtitle translation succeeded via %s using model %s", provider.BaseURL, provider.Model)
}

func locateDeepSeekTestConfig(t *testing.T) string {
	t.Helper()

	if configPath := strings.TrimSpace(os.Getenv("CONFIG_FILE")); configPath != "" {
		return configPath
	}

	candidates := []string{
		filepath.FromSlash("../../config.toml"),
		filepath.FromSlash("../../docker/config.toml"),
		"config.toml",
		filepath.FromSlash("docker/config.toml"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	t.Fatalf("no config file found in candidates: %v", candidates)
	return ""
}

func containsHan(text string) bool {
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}
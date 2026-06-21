package tools

import (
	"testing"

	"github.com/zolagz/ytb2bili/pkg/llm"
	storemodel "github.com/zolagz/ytb2bili/pkg/store/model"
)

func TestApplyStoredTranslationSettingsLoadsUserTranslationSettings(t *testing.T) {
	resolved := applyStoredTranslationSettings(TranslationRunConfig{}, map[string]string{
		storemodel.UserSettingKeyTranslationSourceLang: "ja",
		storemodel.UserSettingKeyTranslationTargetLang: "en",
		storemodel.UserSettingKeyTranslationModel:      "gpt-4.1-mini",
	})
	if resolved.SourceLang != "ja" {
		t.Fatalf("expected source lang from db, got %q", resolved.SourceLang)
	}
	if resolved.TargetLang != "en" {
		t.Fatalf("expected target lang from db, got %q", resolved.TargetLang)
	}
	if resolved.ModelName != "gpt-4.1-mini" {
		t.Fatalf("expected model from db, got %q", resolved.ModelName)
	}
}

func TestApplyStoredTranslationSettingsPrefersExplicitRunConfig(t *testing.T) {
	resolved := applyStoredTranslationSettings(TranslationRunConfig{
		SourceLang: "de",
		TargetLang: "fr",
		ModelName:  "custom-model",
	}, map[string]string{
		storemodel.UserSettingKeyTranslationSourceLang: "ja",
		storemodel.UserSettingKeyTranslationTargetLang: "en",
		storemodel.UserSettingKeyTranslationModel:      "gpt-4.1-mini",
	})
	if resolved.SourceLang != "de" {
		t.Fatalf("expected explicit source lang, got %q", resolved.SourceLang)
	}
	if resolved.TargetLang != "fr" {
		t.Fatalf("expected explicit target lang, got %q", resolved.TargetLang)
	}
	if resolved.ModelName != "custom-model" {
		t.Fatalf("expected explicit model, got %q", resolved.ModelName)
	}
}

func TestApplyStoredTranslationSettingsOverridesDefaultModelWithDatabaseModel(t *testing.T) {
	resolved := applyStoredTranslationSettings(TranslationRunConfig{
		ModelName: llm.DefaultTranslationModel,
	}, map[string]string{
		storemodel.UserSettingKeyTranslationModel: "gemini-2.5-flash",
	})
	if resolved.ModelName != "gemini-2.5-flash" {
		t.Fatalf("expected database model to override default, got %q", resolved.ModelName)
	}
}

func TestBuildLLMChatOptionsDoesNotInjectTemperatureByDefault(t *testing.T) {
	chatOpts := buildLLMChatOptions("gpt-5.2", TranslationChatOptions{})
	if chatOpts.Temperature != nil {
		t.Fatalf("expected default temperature to be omitted, got %v", *chatOpts.Temperature)
	}
	if chatOpts.Model != "gpt-5.2" {
		t.Fatalf("expected model to be preserved, got %q", chatOpts.Model)
	}
}

func TestBuildLLMChatOptionsPreservesExplicitTemperature(t *testing.T) {
	temp := float32(1)
	chatOpts := buildLLMChatOptions("gpt-5.2", TranslationChatOptions{Temperature: &temp})
	if chatOpts.Temperature == nil {
		t.Fatal("expected explicit temperature to be preserved")
	}
	if *chatOpts.Temperature != temp {
		t.Fatalf("expected explicit temperature %v, got %v", temp, *chatOpts.Temperature)
	}
}

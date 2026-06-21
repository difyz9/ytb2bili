package handler

import (
	"testing"

	"github.com/zolagz/ytb2bili/internal/workflow"
)

func TestBuildResumeVideoContextReturnsNilWithoutOverrides(t *testing.T) {
	if buildResumeVideoContext(resumeRequest{}) != nil {
		t.Fatal("expected nil context when resume request has no overrides")
	}
}

func TestBuildResumeVideoContextPreservesExplicitTaskChainOverrides(t *testing.T) {
	ctx := buildResumeVideoContext(resumeRequest{
		PreferredResolution: "1080p",
		RestartFromStep:     "SynthesizeSubtitleAudio",
		TaskChainSettings: &workflow.TaskChainSettings{
			DownloadThumbnail:       false,
			Transcribe:              false,
			TranslateSubtitles:      false,
			SynthesizeSubtitleAudio: true,
		},
		SpeechSynthesisConfig: &workflow.SpeechSynthesisConfig{VoiceName: "zh-CN-XiaoxiaoNeural"},
		TranslationConfig:     &workflow.TranslationConfig{ModelName: "deepseek-v4-flash"},
	})

	if ctx == nil {
		t.Fatal("expected resume video context")
	}
	if ctx.PreferredResolution != "1080p" {
		t.Fatalf("expected preferred resolution to be preserved, got %q", ctx.PreferredResolution)
	}
	if ctx.RestartFromStep != "SynthesizeSubtitleAudio" {
		t.Fatalf("expected restart step to be preserved, got %q", ctx.RestartFromStep)
	}
	if ctx.TaskChainSettings == nil {
		t.Fatal("expected task chain settings")
	}
	if ctx.TaskChainSettings.Transcribe {
		t.Fatal("expected transcribe to remain disabled")
	}
	if ctx.TaskChainSettings.TranslateSubtitles {
		t.Fatal("expected translate_subtitles to remain disabled")
	}
	if !ctx.TaskChainSettings.SynthesizeSubtitleAudio {
		t.Fatal("expected synthesize_subtitle_audio to remain enabled")
	}
	if ctx.SpeechSynthesisConfig == nil || ctx.SpeechSynthesisConfig.VoiceName != "zh-CN-XiaoxiaoNeural" {
		t.Fatal("expected speech synthesis override to be preserved")
	}
	if ctx.TranslationConfig == nil || ctx.TranslationConfig.ModelName != "deepseek-v4-flash" {
		t.Fatal("expected translation override to be preserved")
	}
}
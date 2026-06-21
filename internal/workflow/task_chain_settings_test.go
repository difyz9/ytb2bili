package workflow

import "testing"

func TestNormalizeTaskChainSettingsKeepsExplicitVoiceOnlyMode(t *testing.T) {
	settings := NormalizeTaskChainSettings(&TaskChainSettings{
		DownloadThumbnail:       false,
		Transcribe:              false,
		TranslateSubtitles:      false,
		SynthesizeSubtitleAudio: true,
	})

	if settings == nil {
		t.Fatal("expected normalized settings")
	}
	if settings.Transcribe {
		t.Fatal("expected transcribe to remain disabled")
	}
	if settings.TranslateSubtitles {
		t.Fatal("expected translation to remain disabled")
	}
	if !settings.SynthesizeSubtitleAudio {
		t.Fatal("expected voice-only synth flag to remain enabled")
	}
}

func TestNormalizeTaskChainSettingsKeepsExplicitTranslationOnlyMode(t *testing.T) {
	settings := NormalizeTaskChainSettings(&TaskChainSettings{
		DownloadThumbnail:       false,
		Transcribe:              false,
		TranslateSubtitles:      true,
		SynthesizeSubtitleAudio: false,
	})

	if settings == nil {
		t.Fatal("expected normalized settings")
	}
	if settings.Transcribe {
		t.Fatal("expected transcribe to remain disabled")
	}
	if !settings.TranslateSubtitles {
		t.Fatal("expected translation-only flag to remain enabled")
	}
	if settings.SynthesizeSubtitleAudio {
		t.Fatal("expected synthesize_subtitle_audio to remain disabled")
	}
}
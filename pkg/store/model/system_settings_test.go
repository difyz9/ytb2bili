package model

import "testing"

func TestSystemSettingsApplyPatch(t *testing.T) {
	settings := DefaultSystemSettings()

	err := settings.ApplySettingsPatch(map[string]string{
		SystemSettingKeyYouTubeFeedSyncEnabled:  "0",
		SystemSettingKeyYouTubeFeedSyncInterval: "180",
		SystemSettingKeyYouTubeFeedSyncLookback: "30",
	})
	if err != nil {
		t.Fatalf("ApplySettingsPatch() error = %v", err)
	}

	if settings.YouTubeFeedSyncEnabled {
		t.Fatalf("expected sync to be disabled")
	}

	if settings.YouTubeFeedSyncIntervalMinutes != 180 {
		t.Fatalf("expected interval 180, got %d", settings.YouTubeFeedSyncIntervalMinutes)
	}

	if settings.YouTubeFeedSyncLookbackDays != 30 {
		t.Fatalf("expected lookback 30, got %d", settings.YouTubeFeedSyncLookbackDays)
	}
}

func TestSystemSettingsApplyPatchRejectsUnsupportedInterval(t *testing.T) {
	settings := DefaultSystemSettings()

	err := settings.ApplySettingsPatch(map[string]string{
		SystemSettingKeyYouTubeFeedSyncInterval: "17",
	})
	if err == nil {
		t.Fatal("expected error for unsupported interval")
	}
}

func TestSystemSettingsApplyPatchRejectsUnsupportedLookback(t *testing.T) {
	settings := DefaultSystemSettings()

	err := settings.ApplySettingsPatch(map[string]string{
		SystemSettingKeyYouTubeFeedSyncLookback: "5",
	})
	if err == nil {
		t.Fatal("expected error for unsupported lookback")
	}
}
package model

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	SystemSettingKeyYouTubeFeedSyncEnabled  = "youtube_feed_sync_enabled"
	SystemSettingKeyYouTubeFeedSyncInterval = "youtube_feed_sync_interval_minutes"
	SystemSettingKeyYouTubeFeedSyncLookback = "youtube_feed_sync_lookback_days"

	DefaultYouTubeFeedSyncIntervalMinutes = 60
	DefaultYouTubeFeedSyncLookbackDays    = 7
)

var allowedYouTubeFeedSyncIntervals = map[int]struct{}{
	15:   {},
	30:   {},
	60:   {},
	120:  {},
	180:  {},
	360:  {},
	720:  {},
	1440: {},
}

var allowedYouTubeFeedSyncLookbackDays = map[int]struct{}{
	1:  {},
	3:  {},
	7:  {},
	14: {},
	30: {},
	90: {},
}

type SystemSettings struct {
	BaseModel
	SingletonKey                   string `gorm:"uniqueIndex;size:64;not null" json:"singleton_key"`
	YouTubeFeedSyncEnabled         bool   `gorm:"default:true" json:"youtube_feed_sync_enabled"`
	YouTubeFeedSyncIntervalMinutes int    `gorm:"default:60" json:"youtube_feed_sync_interval_minutes"`
	YouTubeFeedSyncLookbackDays    int    `gorm:"default:7" json:"youtube_feed_sync_lookback_days"`
}

func (SystemSettings) TableName() string {
	return "tb_system_settings"
}

func DefaultSystemSettings() *SystemSettings {
	return &SystemSettings{
		SingletonKey:                   "default",
		YouTubeFeedSyncEnabled:         true,
		YouTubeFeedSyncIntervalMinutes: DefaultYouTubeFeedSyncIntervalMinutes,
		YouTubeFeedSyncLookbackDays:    DefaultYouTubeFeedSyncLookbackDays,
	}
}

func IsAllowedYouTubeFeedSyncIntervalMinutes(value int) bool {
	_, ok := allowedYouTubeFeedSyncIntervals[value]
	return ok
}

func NormalizeYouTubeFeedSyncIntervalMinutes(value int) int {
	if IsAllowedYouTubeFeedSyncIntervalMinutes(value) {
		return value
	}
	return DefaultYouTubeFeedSyncIntervalMinutes
}

func IsAllowedYouTubeFeedSyncLookbackDays(value int) bool {
	_, ok := allowedYouTubeFeedSyncLookbackDays[value]
	return ok
}

func NormalizeYouTubeFeedSyncLookbackDays(value int) int {
	if IsAllowedYouTubeFeedSyncLookbackDays(value) {
		return value
	}
	return DefaultYouTubeFeedSyncLookbackDays
}

func (settings *SystemSettings) ToSettingsMap() map[string]string {
	return map[string]string{
		SystemSettingKeyYouTubeFeedSyncEnabled:  boolToSettingValue(settings.YouTubeFeedSyncEnabled),
		SystemSettingKeyYouTubeFeedSyncInterval: strconv.Itoa(NormalizeYouTubeFeedSyncIntervalMinutes(settings.YouTubeFeedSyncIntervalMinutes)),
		SystemSettingKeyYouTubeFeedSyncLookback: strconv.Itoa(NormalizeYouTubeFeedSyncLookbackDays(settings.YouTubeFeedSyncLookbackDays)),
	}
}

func (settings *SystemSettings) ApplySettingsPatch(patch map[string]string) error {
	for key, rawValue := range patch {
		value := strings.TrimSpace(rawValue)
		switch key {
		case SystemSettingKeyYouTubeFeedSyncEnabled:
			enabled, err := parseBoolSettingValue(value)
			if err != nil {
				return err
			}
			settings.YouTubeFeedSyncEnabled = enabled
		case SystemSettingKeyYouTubeFeedSyncInterval:
			minutes, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("invalid youtube feed sync interval: %s", value)
			}
			if !IsAllowedYouTubeFeedSyncIntervalMinutes(minutes) {
				return fmt.Errorf("unsupported youtube feed sync interval: %d", minutes)
			}
			settings.YouTubeFeedSyncIntervalMinutes = minutes
		case SystemSettingKeyYouTubeFeedSyncLookback:
			days, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("invalid youtube feed sync lookback days: %s", value)
			}
			if !IsAllowedYouTubeFeedSyncLookbackDays(days) {
				return fmt.Errorf("unsupported youtube feed sync lookback days: %d", days)
			}
			settings.YouTubeFeedSyncLookbackDays = days
		default:
			return fmt.Errorf("unsupported system setting key: %s", key)
		}
	}

	settings.YouTubeFeedSyncIntervalMinutes = NormalizeYouTubeFeedSyncIntervalMinutes(settings.YouTubeFeedSyncIntervalMinutes)
	settings.YouTubeFeedSyncLookbackDays = NormalizeYouTubeFeedSyncLookbackDays(settings.YouTubeFeedSyncLookbackDays)
	if strings.TrimSpace(settings.SingletonKey) == "" {
		settings.SingletonKey = "default"
	}
	return nil
}
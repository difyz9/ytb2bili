package service

import (
	"context"
	"fmt"

	"github.com/zolagz/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type SystemSettingsClient struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewSystemSettingsClient(db *gorm.DB, logger *zap.Logger) *SystemSettingsClient {
	return &SystemSettingsClient{db: db, logger: logger}
}

func (c *SystemSettingsClient) IsEnabled() bool {
	return c != nil && c.db != nil
}

func (c *SystemSettingsClient) GetSettings(ctx context.Context) (map[string]string, error) {
	record, err := c.GetSettingsRecord(ctx)
	if err != nil {
		return nil, err
	}
	return record.ToSettingsMap(), nil
}

func (c *SystemSettingsClient) GetSettingsRecord(ctx context.Context) (*model.SystemSettings, error) {
	defaults := model.DefaultSystemSettings()
	if !c.IsEnabled() {
		return defaults, nil
	}

	var record model.SystemSettings
	err := c.db.WithContext(ctx).Where("singleton_key = ?", defaults.SingletonKey).First(&record).Error
	if err == gorm.ErrRecordNotFound {
		return defaults, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query system settings: %w", err)
	}

	record.YouTubeFeedSyncIntervalMinutes = model.NormalizeYouTubeFeedSyncIntervalMinutes(record.YouTubeFeedSyncIntervalMinutes)
	record.YouTubeFeedSyncLookbackDays = model.NormalizeYouTubeFeedSyncLookbackDays(record.YouTubeFeedSyncLookbackDays)
	if record.SingletonKey == "" {
		record.SingletonKey = defaults.SingletonKey
	}
	return &record, nil
}

func (c *SystemSettingsClient) UpdateSettings(ctx context.Context, patch map[string]string) (*model.SystemSettings, error) {
	record, err := c.GetSettingsRecord(ctx)
	if err != nil {
		return nil, err
	}
	if err := record.ApplySettingsPatch(patch); err != nil {
		return nil, err
	}

	if record.ID == 0 {
		if err := c.db.WithContext(ctx).Create(record).Error; err != nil {
			return nil, fmt.Errorf("create system settings: %w", err)
		}
		return record, nil
	}

	if err := c.db.WithContext(ctx).Save(record).Error; err != nil {
		return nil, fmt.Errorf("save system settings: %w", err)
	}
	return record, nil
}
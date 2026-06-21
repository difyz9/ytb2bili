package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/zolagz/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type UserSettingsClient struct {
	db     *gorm.DB
	logger *zap.Logger
}

type UserSettingsUpdateError struct {
	StatusCode int
	Message    string
}

func (e *UserSettingsUpdateError) Error() string {
	if e == nil {
		return "update user settings failed"
	}
	return strings.TrimSpace(e.Message)
}

func NewUserSettingsClient(db *gorm.DB, logger *zap.Logger) *UserSettingsClient {
	return &UserSettingsClient{db: db, logger: logger}
}

func (c *UserSettingsClient) IsEnabled() bool {
	return c != nil && c.db != nil
}

func (c *UserSettingsClient) GetSettings(ctx context.Context, userID string) (map[string]string, error) {
	record, err := c.GetSettingsRecord(ctx, userID)
	if err != nil {
		return nil, err
	}
	return record.ToSettingsMap(), nil
}

func (c *UserSettingsClient) GetSettingsRecord(ctx context.Context, userID string) (*model.UserSettings, error) {
	trimmedUserID := strings.TrimSpace(userID)
	if !c.IsEnabled() || trimmedUserID == "" {
		return &model.UserSettings{
			UserID:                    trimmedUserID,
			AutoUploadIntervalMinutes: model.DefaultAutoUploadIntervalMinutes,
			ExtraSettings:             "{}",
		}, nil
	}

	var record model.UserSettings
	err := c.db.WithContext(ctx).Where("user_id = ?", trimmedUserID).First(&record).Error
	if err == gorm.ErrRecordNotFound {
		return &model.UserSettings{
			UserID:                    trimmedUserID,
			AutoUploadIntervalMinutes: model.DefaultAutoUploadIntervalMinutes,
			ExtraSettings:             "{}",
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query user settings: %w", err)
	}

	record.AutoUploadIntervalMinutes = model.NormalizeAutoUploadIntervalMinutes(record.AutoUploadIntervalMinutes)
	return &record, nil
}

func (c *UserSettingsClient) UpdateSettings(ctx context.Context, userID string, patch map[string]string) (*model.UserSettings, error) {
	if err := c.validatePatchAccess(ctx, userID, patch); err != nil {
		return nil, err
	}

	record, err := c.GetSettingsRecord(ctx, userID)
	if err != nil {
		return nil, err
	}
	if err := record.ApplySettingsPatch(patch); err != nil {
		return nil, err
	}

	if record.ID == 0 {
		if err := c.db.WithContext(ctx).Create(record).Error; err != nil {
			return nil, fmt.Errorf("create user settings: %w", err)
		}
		return record, nil
	}

	if err := c.db.WithContext(ctx).Save(record).Error; err != nil {
		return nil, fmt.Errorf("save user settings: %w", err)
	}
	return record, nil
}

func (c *UserSettingsClient) validatePatchAccess(ctx context.Context, userID string, patch map[string]string) error {
	// In open-source mode, all settings are freely configurable by users
	return nil
}

func parseUserSettingBool(value string) (enabled bool, parsed bool, err error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true":
		return true, true, nil
	case "0", "false", "":
		return false, true, nil
	default:
		return false, false, fmt.Errorf("invalid bool setting: %s", value)
	}
}


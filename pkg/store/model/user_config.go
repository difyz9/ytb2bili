package model

import (
	"time"
)

// UserPreference 用户个人偏好
type UserPreference struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	UserID    uint      `gorm:"uniqueIndex;not null" json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// 通知设置
	EmailNotificationsEnabled bool   `gorm:"default:true" json:"email_notifications_enabled"`
	NotificationEmail         string `gorm:"size:255" json:"notification_email,omitempty"`

	// 任务默认设置
	DefaultAutoUpload    bool   `gorm:"default:true" json:"default_auto_upload"`
	DefaultUploadDelay   int    `gorm:"default:10" json:"default_upload_delay"`
	DefaultSubtitleDelay int    `gorm:"default:10" json:"default_subtitle_delay"`
	DefaultCopyright     int    `gorm:"default:2" json:"default_copyright"`
	DefaultSource        string `gorm:"default:'YouTube'" json:"default_source"`
	DefaultTid           int    `gorm:"default:122" json:"default_tid"`

	// 界面设置
	Theme        string `gorm:"default:'light';size:20" json:"theme"` // light/dark
	Language     string `gorm:"default:'zh';size:10" json:"language"` // zh/en
	ItemsPerPage int    `gorm:"default:20" json:"items_per_page"`
	ShowAdvanced bool   `gorm:"default:false" json:"show_advanced"`

	// 隐私设置
	EnableAnalytics bool `gorm:"default:false" json:"enable_analytics"`
}

func (UserPreference) TableName() string {
	return "tb_user_preferences"
}

package model

import "time"

const (
	BiliSubtitleLanguageZh = "zh-CN"
	BiliSubtitleLanguageEn = "en"

	BiliSubtitleStatusPending  = "pending"
	BiliSubtitleStatusUploaded = "uploaded"
	BiliSubtitleStatusFailed   = "failed"
	BiliSubtitleStatusMissing  = "missing"
)

// BiliSubtitleUpload tracks the upload state of each expected Bilibili subtitle file.
type BiliSubtitleUpload struct {
	BaseModel
	VideoID       string     `gorm:"size:100;not null;uniqueIndex:idx_bili_subtitle_video_lang,priority:1;index" json:"video_id"`
	UserID        string     `gorm:"size:128;not null;index" json:"user_id"`
	BiliBVID      string     `gorm:"column:bili_bvid;size:50;not null;index" json:"bili_bvid"`
	Language      string     `gorm:"size:16;not null;uniqueIndex:idx_bili_subtitle_video_lang,priority:2" json:"language"`
	FileName      string     `gorm:"size:255;not null" json:"file_name"`
	FilePath      string     `gorm:"size:500;not null" json:"file_path"`
	Status        string     `gorm:"size:20;not null;default:'pending';index" json:"status"`
	AttemptCount  int        `gorm:"not null;default:0" json:"attempt_count"`
	LastError     string     `gorm:"type:text" json:"last_error,omitempty"`
	LastCheckedAt *time.Time `json:"last_checked_at,omitempty"`
	UploadedAt    *time.Time `json:"uploaded_at,omitempty"`
}

func (BiliSubtitleUpload) TableName() string {
	return "tb_bili_subtitle_uploads"
}
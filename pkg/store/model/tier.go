// Package membership 会员系统核心模块
package model

import "time"

// Tier 会员等级
type Tier string

const (
	TierFree       Tier = "free"
	TierBasic      Tier = "basic"
	TierStandard   Tier = "standard"
	TierPro        Tier = "pro"
	TierEnterprise Tier = "enterprise"
)

// LicenseActivation 许可证激活记录
type LicenseActivation struct {
	ID          int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	LicenseKey  string     `gorm:"type:varchar(64);uniqueIndex;not null" json:"license_key"`
	UserID      string     `gorm:"type:varchar(64);index;not null" json:"user_id"`
	Tier        Tier       `gorm:"type:varchar(32);not null" json:"tier"`
	Plan        string     `gorm:"type:varchar(32);not null" json:"plan"`
	ExpiresAt   *time.Time `gorm:"type:datetime" json:"expires_at,omitempty"`
	ActivatedAt time.Time  `gorm:"type:datetime;not null" json:"activated_at"`
	CreatedAt   time.Time  `gorm:"type:datetime;not null;autoCreateTime" json:"created_at"`
}

func (LicenseActivation) TableName() string {
	return "tb_license_activations"
}

// UserMembership 用户会员信息
type UserMembership struct {
	UserID    string    `gorm:"type:varchar(64);primaryKey" json:"user_id"`
	Tier      Tier      `gorm:"type:varchar(32);not null;default:free;index" json:"tier"`
	ExpiresAt time.Time `gorm:"type:datetime;index" json:"expires_at"`
	CreatedAt time.Time `gorm:"type:datetime;not null;autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"type:datetime;not null;autoUpdateTime" json:"updated_at"`
}

func (UserMembership) TableName() string {
	return "tb_user_memberships"
}

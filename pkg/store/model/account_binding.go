package model

import (
	"encoding/json"
	"fmt"
	"time"
)

// Platform 平台类型
type Platform string

const (
	PlatformBilibili Platform = "bilibili" // B站
	PlatformDouyin   Platform = "douyin"   // 抖音
	PlatformYoutube  Platform = "youtube"  // YouTube
	PlatformKuaishou Platform = "kuaishou" // 快手
)

// BindingStatus 绑定状态
type BindingStatus string

const (
	BindingStatusPending BindingStatus = "pending" // 等待绑定
	BindingStatusBound   BindingStatus = "bound"   // 已绑定
	BindingStatusExpired BindingStatus = "expired" // 已过期
	BindingStatusUnbound BindingStatus = "unbound" // 已解绑
)

// AccountBinding 账号绑定表 - 多平台账号统一管理
type AccountBinding struct {
	BaseModel
	UserID       string        `gorm:"size:128;index;not null" json:"user_id"` // 用户ID
	Platform     Platform      `gorm:"size:50;not null" json:"platform"`       // 平台类型
	PlatformUID  string        `gorm:"size:100;not null" json:"platform_uid"`  // 平台用户ID
	Username     string        `gorm:"size:255" json:"username"`               // 平台用户名
	Avatar       string        `gorm:"size:500" json:"avatar"`                 // 平台头像
	AccessToken  string        `gorm:"type:text" json:"-"`                     // 访问令牌
	RefreshToken string        `gorm:"type:text" json:"-"`                     // 刷新令牌
	ExpiresAt    *time.Time    `json:"expires_at"`                             // 令牌过期时间
	Status       BindingStatus `gorm:"size:20;default:pending" json:"status"`  // 绑定状态

	// 通用扩展字段
	IsPrimary  bool       `gorm:"column:is_primary;default:false;index" json:"is_primary"` // 是否为主账号（用于B站等平台）
	LastUsedAt *time.Time `gorm:"column:last_used_at" json:"last_used_at"`                 // 最后使用时间
	Cookies    string     `gorm:"column:cookies;type:text" json:"-"`                       // 加密的Cookies（B站专用）

	// 平台特定数据（JSON格式存储）
	// 示例：{"bili_mid": 123456, "bili_level": 6, "bili_vip": true}
	PlatformData *string `gorm:"column:platform_data;type:json" json:"platform_data,omitempty"` // 平台特定扩展数据（使用指针类型，nil时存储为NULL）

	// 临时二维码相关字段（仅在绑定流程中使用）
	//QRCode    string `gorm:"size:500" json:"qr_code,omitempty"`     // 绑定二维码URL
	//QRCodeKey string `gorm:"size:255;index" json:"qr_code_key,omitempty"` // 二维码密钥
}

// TableName 指定表名
func (AccountBinding) TableName() string {
	return "tb_account_bindings"
}

// IsExpired 检查令牌是否过期
func (ab *AccountBinding) IsExpired() bool {
	if ab.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*ab.ExpiresAt)
}

// IsActive 检查绑定是否有效
func (ab *AccountBinding) IsActive() bool {
	return ab.Status == BindingStatusBound && !ab.IsExpired()
}

// BiliPlatformData B站平台特定数据结构
type BiliPlatformData struct {
	BiliMid   int64  `json:"bili_mid"`   // B站用户MID
	BiliLevel int    `json:"bili_level"` // 用户等级
	BiliVip   bool   `json:"bili_vip"`   // 是否大会员
	BiliSign  string `json:"bili_sign"`  // 个性签名
}

// GetBiliData 获取B站特定数据
func (ab *AccountBinding) GetBiliData() (*BiliPlatformData, error) {
	if ab.Platform != PlatformBilibili || ab.PlatformData == nil || *ab.PlatformData == "" {
		return nil, nil
	}

	var data BiliPlatformData
	if err := json.Unmarshal([]byte(*ab.PlatformData), &data); err != nil {
		return nil, err
	}
	return &data, nil
}

// SetBiliData 设置B站特定数据
func (ab *AccountBinding) SetBiliData(data *BiliPlatformData) error {
	if data == nil {
		ab.PlatformData = nil
		return nil
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	jsonStr := string(jsonData)
	ab.PlatformData = &jsonStr
	return nil
}

// parseIntSafe 安全地解析字符串为int64
func parseIntSafe(s string) (int64, error) {
	var result int64
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}

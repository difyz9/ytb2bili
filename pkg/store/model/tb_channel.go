package model

// Channel YouTube/B站频道信息
type Channel struct {
	BaseModel
	UserID          string `gorm:"size:128;index;not null" json:"user_id"`          // 用户ID
	Platform        string `gorm:"size:20;not null" json:"platform"`                // 平台: youtube/bilibili
	ChannelID       string `gorm:"size:255;uniqueIndex;not null" json:"channel_id"` // 频道ID
	Title           string `gorm:"size:500" json:"title"`                           // 频道标题
	Description     string `gorm:"type:text" json:"description"`                    // 频道描述
	CustomURL       string `gorm:"size:500" json:"custom_url"`                      // 自定义URL
	ThumbnailURL    string `gorm:"size:500" json:"thumbnail_url"`                   // 缩略图URL
	SubscriberCount int    `json:"subscriber_count"`                                // 订阅人数
}

// TableName 指定表名
func (Channel) TableName() string {
	return "tb_channels"
}

package model

import (
	"time"

	"gorm.io/gorm"
)

// BaseModel 基础模型
type BaseModel struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// AudioResult 音频处理结果
type AudioResult struct {
	SID            int     `json:"sid"`
	Text           string  `json:"text"`
	TranslatedText string  `json:"translated_text,omitempty"`
	AudioURL       string  `json:"audio_url"`
	Language       string  `json:"language"`
	Duration       float64 `json:"duration"`
}

// TranslationSettings 翻译设置
type TranslationSettings struct {
	SourceLanguage string  `json:"source_language"`
	TargetLanguage string  `json:"target_language"`
	Service        string  `json:"service"`
	Gender         string  `json:"gender"`
	Tier           string  `json:"tier"`
	VoiceName      string  `json:"voice_name"`
	VoiceSpeed     float64 `json:"voice_speed"`
}

// VideoProcessingRequest 视频处理请求（根据用户提供的JSON格式）
type VideoProcessingRequest struct {
	VideoID             string              `json:"video_id"`
	Platform            string              `json:"platform"`
	Subtitles           []SubtitleItem      `json:"subtitles"`
	TranslationSettings TranslationSettings `json:"translation_settings"`
}

// User 用户模型 - 核心用户信息
type User struct {
	BaseModel
	FirebaseUID string     `gorm:"uniqueIndex;size:128;not null" json:"firebase_uid"` // 用户唯一标识（gorm列名保留兼容）
	Email       string     `gorm:"uniqueIndex;size:100" json:"email"`                 // 邮箱（可选）
	Username    string     `gorm:"size:100" json:"username"`                          // 用户名
	Avatar      string     `gorm:"size:500" json:"avatar"`                            // 头像URL
	Role        string     `gorm:"size:20;default:user;index" json:"role"`            // 用户角色: admin/user
	Status      int        `gorm:"default:1" json:"status"`                           // 状态: 1-启用 0-禁用
	LastLoginAt *time.Time `json:"last_login_at"`                                     // 最后登录时间
}

// TableName 指定表名
func (User) TableName() string {
	return "tb_users"
}

type SubtitleItem struct {
	SID      int     `json:"sid" gorm:"column:sid"`           // 字幕ID
	From     float64 `json:"from" gorm:"column:from_time"`    // 开始时间
	To       float64 `json:"to" gorm:"column:to_time"`        // 结束时间
	Text     string  `json:"text" gorm:"column:content"`      // 字幕内容（兼容用户格式）
	Content  string  `json:"content" gorm:"column:content"`   // 字幕内容（兼容数据库格式）
	Location int     `json:"location" gorm:"column:location"` // 位置信息
}

// SavedVideoSubtitle 用户提交的字幕条目（用于API接收）
type SavedVideoSubtitle struct {
	Text     string  `json:"text"`     // 字幕文本
	Duration float64 `json:"duration"` // 持续时间
	Offset   float64 `json:"offset"`   // 偏移时间
	Lang     string  `json:"lang"`     // 语言
}

// Video 视频信息表 - 核心视频元数据
type Video struct {
	BaseModel
	UserID      string     `gorm:"size:128;index;not null" json:"user_id"`        // 用户ID
	VideoID     string     `gorm:"size:100;uniqueIndex;not null" json:"video_id"` // 视频ID（YouTube ID等）
	Platform    string     `gorm:"size:20;index" json:"platform"`                 // 来源平台: youtube/bilibili等
	URL         string     `gorm:"size:500;not null" json:"url"`                  // 原始视频URL
	Title       string     `gorm:"size:500" json:"title"`                         // 视频标题
	Description string     `gorm:"type:text" json:"description"`                  // 视频描述
	Thumbnail   string     `gorm:"size:500" json:"thumbnail"`                     // 缩略图URL
	Duration    float64    `gorm:"type:float" json:"duration"`                    // 视频时长（秒）
	Status      string     `gorm:"size:20;index" json:"status"`                   // 处理状态: 001=待处理/002=处理中/003=已完成/004=失败
	RetryCount  int        `gorm:"default:0" json:"retry_count"`                  // 重试次数
	PublishedAt *time.Time `gorm:"column:published_at;index" json:"published_at"` // 视频发布时间（YouTube等平台的原始发布时间）

	// AI生成的元数据
	GeneratedTitle  string `gorm:"size:500" json:"generated_title"`                          // AI生成的标题
	GeneratedDesc   string `gorm:"type:text" json:"generated_desc"`                          // AI生成的描述
	GeneratedTags   string `gorm:"size:500" json:"generated_tags"`                           // AI生成的标签（逗号分隔）
	RecommendedTags string `gorm:"size:500" json:"recommended_tags"`                         // B站投稿前推荐标签（逗号分隔）
	ChannelId       string `gorm:"size:64;column:channel_id;comment:频道ID" json:"channel_id"` // 频道ID
	SortNum         int    `gorm:"column:sort_num;default:0" json:"sortNum"`
	// B站上传结果
	BiliBVID             string `gorm:"column:bili_bvid;size:50;index" json:"bili_bvid"`                           // B站BVID
	BiliAID              int64  `gorm:"column:bili_aid;index" json:"bili_aid"`                                     // B站AID
	BiliSubtitleUploaded bool   `gorm:"column:bili_subtitle_uploaded;default:false" json:"bili_subtitle_uploaded"` // 字幕是否已上传到B站

	// 文件路径
	VideoPath           string `gorm:"column:video_path;size:500" json:"video_path"`                    // 本地视频文件路径
	VideoSizeBytes      int64  `gorm:"column:video_size_bytes;default:0" json:"video_size_bytes"`       // 本地视频文件大小（字节）
	SubtitlePath        string `gorm:"column:subtitle_path;size:500" json:"subtitle_path"`              // 字幕文件路径
	PreferredResolution string `gorm:"column:preferred_resolution;size:20" json:"preferred_resolution"` // 期望下载分辨率: best/720p/1080p/1440p/2160p
	SpeechVoiceName     string `gorm:"column:speech_voice_name;size:100" json:"speech_voice_name"`      // 本次任务使用的字幕配音音色
	TaskChainSettings   string `gorm:"column:task_chain_settings;type:text" json:"-"`                   // 提交时任务链快照

	// 用户提交的额外字段
	OperationType string `gorm:"column:operation_type;size:50" json:"operation_type"` // 操作类型
	Subtitles     string `gorm:"column:subtitles;type:mediumtext" json:"subtitles"`   // 字幕JSON数据（mediumtext，最大16MB）
	PlaylistID    string `gorm:"column:playlist_id;size:100" json:"playlist_id"`      // 播放列表ID
	Timestamp     int64  `gorm:"column:timestamp" json:"timestamp"`                   // 时间戳
	SavedAt       string `gorm:"column:saved_at;size:50" json:"saved_at"`             // 保存时间
}

// TableName 指定表名
func (Video) TableName() string {
	return "tb_videos"
}

// App 应用/客户端模型 - API密钥管理
type App struct {
	BaseModel
	AppID     string `gorm:"uniqueIndex;size:64;not null" json:"app_id"` // 应用ID
	AppSecret string `gorm:"size:128;not null" json:"-"`                 // 应用密钥
	Name      string `gorm:"size:100;not null" json:"name"`              // 应用名称
	OwnerID   string `gorm:"size:128;index;not null" json:"owner_id"`    // 所属用户ID (关联User.FirebaseUID，无物理外键)
	Status    int    `gorm:"default:1" json:"status"`                    // 状态: 1-启用 0-禁用
}

// TableName 指定表名
func (App) TableName() string {
	return "tb_apps"
}

// UserToken Token黑名单/会话管理
type UserToken struct {
	BaseModel
	UserID    string    `gorm:"size:128;index;not null" json:"user_id"` // 用户ID
	TokenHash string    `gorm:"uniqueIndex;size:64;not null" json:"-"`  // Token哈希
	ExpiresAt time.Time `json:"expires_at"`                             // 过期时间
	IsRevoked bool      `gorm:"default:false" json:"is_revoked"`        // 是否已撤销
}

// TableName 指定表名
func (UserToken) TableName() string {
	return "tb_user_tokens"
}

// EmailVerification 邮箱验证码
type EmailVerification struct {
	BaseModel
	Email     string    `gorm:"index;size:100;not null" json:"email"` // 邮箱
	Code      string    `gorm:"size:10;not null" json:"-"`            // 验证码
	Type      string    `gorm:"size:20;not null" json:"type"`         // 类型: register/login/reset
	ExpiresAt time.Time `gorm:"not null" json:"expires_at"`           // 过期时间
	Used      bool      `gorm:"default:false" json:"used"`            // 是否已使用
}

// TableName 指定表名
func (EmailVerification) TableName() string {
	return "tb_email_verifications"
}

// TbSubscription 用户订阅频道
type TbSubscription struct {
	BaseModel
	UserID              string    `gorm:"size:128;index:idx_user_channel;not null" json:"user_id"`    // 用户ID
	ChannelID           string    `gorm:"size:255;index:idx_user_channel;not null" json:"channel_id"` // 频道ID
	Platform            string    `gorm:"size:20;not null" json:"platform"`                           // 平台: youtube/bilibili等
	ChannelTitle        string    `gorm:"size:500" json:"channel_title"`                              // 频道标题
	ChannelDescription  string    `gorm:"type:text" json:"channel_description"`                       // 频道描述
	ChannelThumbnailURL string    `gorm:"size:500" json:"channel_thumbnail_url"`                      // 频道缩略图URL
	ChannelCustomURL    string    `gorm:"size:500" json:"channel_custom_url"`                         // 频道自定义URL
	SubscribedAt        time.Time `json:"subscribed_at"`                                              // 订阅时间
	Status              string    `gorm:"size:20;default:active" json:"status"`                       // 状态: active/inactive
	SyncedAt            time.Time `json:"synced_at"`                                                  // 最后同步时间
}

// TableName 指定表名
func (TbSubscription) TableName() string {
	return "tb_subscription"
}

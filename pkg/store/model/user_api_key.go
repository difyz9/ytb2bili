package model

// UserAPIKey mirrors a user-created remote API key into the local GORM store.
type UserAPIKey struct {
	BaseModel
	UserID                string `gorm:"size:128;not null;uniqueIndex:idx_user_api_keys_user_remote,priority:1;index" json:"user_id"`
	RemoteKeyID           string `gorm:"size:128;not null;uniqueIndex:idx_user_api_keys_user_remote,priority:2" json:"remote_key_id"`
	Name                  string `gorm:"size:255;not null" json:"name"`
	KeyPrefix             string `gorm:"size:32;index" json:"key_prefix"`
	SecretEncrypted       string `gorm:"type:text" json:"-"`
	Active                bool   `gorm:"default:true;index" json:"active"`
	LastUsedAtMillis      *int64 `gorm:"column:last_used_at_ms" json:"last_used_at"`
	RemoteCreatedAtMillis int64  `gorm:"column:remote_created_at_ms;not null" json:"created_at"`
	RemoteUpdatedAtMillis int64  `gorm:"column:remote_updated_at_ms;not null" json:"updated_at"`
}

func (UserAPIKey) TableName() string {
	return "tb_user_api_keys"
}

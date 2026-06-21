package tools

import "context"

// UserSettingsProvider 是 pkg/tools 需要的用户设置接口。
// 由 internal/service.UserSettingsClient 实现。
// 定义为接口以避免 pkg/tools → internal/service 的层违反。
type UserSettingsProvider interface {
	GetSettings(ctx context.Context, userID string) (map[string]string, error)
	IsEnabled() bool
}

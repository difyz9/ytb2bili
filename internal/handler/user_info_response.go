package handler

// UserInfoResponse 统一的用户信息响应结构。
type UserInfoResponse struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	PhotoURL    string `json:"photo_url"`
	Provider    string `json:"provider"`
}

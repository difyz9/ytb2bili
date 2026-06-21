package model

// 视频处理状态常量
const (
	VideoStatusPending    = "001" // 待处理
	VideoStatusProcessing = "002" // 处理中
	VideoStatusCompleted  = "003" // 已完成
	VideoStatusFailed     = "004" // 失败
	VideoStatusPaused     = "paused"
)

// VideoStatusText 返回状态码对应的文本描述
func VideoStatusText(status string) string {
	switch status {
	case VideoStatusPending:
		return "待处理"
	case VideoStatusProcessing:
		return "处理中"
	case VideoStatusCompleted:
		return "已完成"
	case VideoStatusFailed:
		return "失败"
	case VideoStatusPaused:
		return "已停止"
	default:
		return "未知状态"
	}
}

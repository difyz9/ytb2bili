package tools

import (
	"context"

	"github.com/difyz9/ytb2bili/internal/core"
)

// UploadToBiliTool 上传到 Bilibili
// 将处理好的视频上传到 Bilibili 平台，包含完整的元数据设置
type UploadToBiliTool struct {
	app     *core.AppServer
	videoId string
}

// NewUploadToBiliTool 创建 Bilibili 上传工具
func NewUploadToBiliTool(app *core.AppServer, videoId string) *UploadToBiliTool {
	return &UploadToBiliTool{
		app:     app,
		videoId: videoId,
	}
}

func (t *UploadToBiliTool) Name() string {
	return "upload_to_bilibili"
}

func (t *UploadToBiliTool) Description() string {
	return `上传视频到 Bilibili 平台。
功能：完整的视频上传流程，包括视频文件、封面、字幕、元数据设置。
输入格式：视频 ID（字符串）或 JSON:
{
  "video_id": "视频 ID (必需)",
  "title": "视频标题 (可选: 未提供则使用已生成的元数据)",
  "description": "视频简介 (可选)",
  "tags": ["标签1", "标签2"] 或 "标签1,标签2" (可选),
  "category_id": 分区 ID (可选: 17-动画, 1-动漫, 3-音乐等),
  "cover_path": "封面图路径 (可选: 未提供则使用缩略图)",
  "subtitle_path": "字幕文件路径 (可选)",
  "publish_time": "定时发布时间 (可选: ISO8601 格式, 如 2025-12-11T20:00:00+08:00)",
  "is_original": 是否原创 (可选: true/false, 默认: false)
}
返回：Bilibili 视频信息 JSON
{
  "bv_id": "BV1xx411c7mD",
  "av_id": 123456789,
  "url": "https://www.bilibili.com/video/BV1xx411c7mD"
}

示例输入1: "abc123"
示例输入2: {"video_id": "abc123", "category_id": 17, "publish_time": "2025-12-11T20:00:00+08:00"}`
}

func (t *UploadToBiliTool) Call(ctx context.Context, input string) (string, error) {

	return "", nil
}

package tools

import (
	"context"

	"github.com/difyz9/ytb2bili/internal/core"
)

// DownloadThumbnailTool 下载视频缩略图
// 从 YouTube 获取视频封面图，支持多种分辨率
type DownloadThumbnailTool struct {
	app     *core.AppServer
	videoId string
}

// NewDownloadThumbnailTool 创建缩略图下载工具
func NewDownloadThumbnailTool(app *core.AppServer, videoId string) *DownloadThumbnailTool {
	return &DownloadThumbnailTool{
		app:     app,
		videoId: videoId,
	}
}

func (t *DownloadThumbnailTool) Name() string {
	return "download_thumbnail"
}

func (t *DownloadThumbnailTool) Description() string {
	return `下载 YouTube 视频的缩略图（封面图）。
功能：获取视频封面，支持最高画质。
输入格式：视频 ID（字符串）或 JSON:
{
  "video_id": "YouTube 视频 ID (必需)",
  "resolution": "分辨率 (可选: maxres, high, medium, default, 默认: maxres)"
}
返回：缩略图文件的本地路径 (如: /data/videos/abc123/thumbnail.jpg)

示例输入1: "dQw4w9WgXcQ"
示例输入2: {"video_id": "dQw4w9WgXcQ", "resolution": "maxres"}`
}

func (t *DownloadThumbnailTool) Call(ctx context.Context, input string) (string, error) {

	return "", nil
}

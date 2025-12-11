package tools

import (
	"context"

	"github.com/difyz9/ytb2bili/internal/core"
)

// DownloadVideoTool 下载 YouTube 视频的工具
// 使用 yt-dlp 下载指定 YouTube 视频到本地存储
type DownloadVideoTool struct {
	app     *core.AppServer
	videoId string
}

// NewDownloadVideoTool 创建视频下载工具
func NewDownloadVideoTool(app *core.AppServer, videoId string) *DownloadVideoTool {
	return &DownloadVideoTool{
		app:     app,
		videoId: videoId,
	}
}

func (t *DownloadVideoTool) Name() string {
	return "download_video"
}

func (t *DownloadVideoTool) Description() string {
	return `下载 YouTube 视频到本地。
功能：使用 yt-dlp 下载视频，支持高清画质选择。
输入格式：视频 ID（字符串）或 JSON:
{
  "video_id": "YouTube 视频 ID (必需)",
  "quality": "视频质量 (可选: best, 1080p, 720p, 默认: best)",
  "format": "视频格式 (可选: mp4, webm, 默认: mp4)"
}
返回：视频文件的本地路径 (如: /data/videos/abc123/video.mp4)

示例输入1: "dQw4w9WgXcQ"
示例输入2: {"video_id": "dQw4w9WgXcQ", "quality": "1080p"}`
}

func (t *DownloadVideoTool) Call(ctx context.Context, input string) (string, error) {

	return "", nil
}
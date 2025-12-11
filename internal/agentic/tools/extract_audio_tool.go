package tools

import (
	"context"

	"github.com/difyz9/ytb2bili/internal/core"
)

// ExtractAudioTool 提取视频音频
// 使用 FFmpeg 从视频文件中提取音频轨道
type ExtractAudioTool struct {
	app     *core.AppServer
	videoId string
}

// NewExtractAudioTool 创建音频提取工具
func NewExtractAudioTool(app *core.AppServer, videoId string) *ExtractAudioTool {
	return &ExtractAudioTool{
		app:     app,
		videoId: videoId,
	}
}

func (t *ExtractAudioTool) Name() string {
	return "extract_audio"
}

func (t *ExtractAudioTool) Description() string {
	return `从视频文件中提取音频轨道。
功能：使用 FFmpeg 提取高质量音频，用于字幕生成或单独发布。
输入格式：视频 ID（字符串）或 JSON:
{
  "video_id": "视频 ID (必需)",
  "format": "音频格式 (可选: mp3, wav, aac, 默认: mp3)",
  "bitrate": "音频比特率 (可选: 128k, 192k, 256k, 默认: 192k)",
  "channels": "声道数 (可选: 1, 2, 默认: 2)"
}
返回：音频文件的本地路径 (如: /data/videos/abc123/audio.mp3)

示例输入1: "abc123"
示例输入2: {"video_id": "abc123", "format": "wav", "bitrate": "256k"}`
返回: 音频文件路径`
}

func (t *ExtractAudioTool) Call(ctx context.Context, input string) (string, error) {

	return "", nil
}

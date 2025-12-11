package tools

import (
	"context"

	"github.com/difyz9/ytb2bili/internal/core"
)

// GenerateSubtitleTool 生成字幕
// 使用 Whisper AI 模型进行语音识别，生成 SRT/VTT 格式字幕
type GenerateSubtitleTool struct {
	app     *core.AppServer
	videoId string
}

// NewGenerateSubtitleTool 创建字幕生成工具
func NewGenerateSubtitleTool(app *core.AppServer, videoId string) *GenerateSubtitleTool {
	return &GenerateSubtitleTool{
		app:     app,
		videoId: videoId,
	}
}

func (t *GenerateSubtitleTool) Name() string {
	return "generate_subtitle"
}

func (t *GenerateSubtitleTool) Description() string {
	return `使用 Whisper AI 模型从音频生成字幕文件。
功能：自动语音识别（ASR），生成带时间轴的字幕，支持多语言检测。
输入格式：视频 ID（字符串）或 JSON:
{
  "video_id": "视频 ID (必需)",
  "language": "语言代码 (可选: en, zh, ja, auto, 默认: auto 自动检测)",
  "model": "Whisper 模型 (可选: tiny, base, small, medium, large, 默认: base)",
  "format": "字幕格式 (可选: srt, vtt, 默认: srt)"
}
返回：字幕文件的本地路径 (如: /data/videos/abc123/subtitle_en.srt)

示例输入1: "abc123"
示例输入2: {"video_id": "abc123", "language": "en", "model": "medium"}`
返回: 字幕文件路径`
}

func (t *GenerateSubtitleTool) Call(ctx context.Context, input string) (string, error) {
	return "", nil
}

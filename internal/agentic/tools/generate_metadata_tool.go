package tools

import (
	"context"

	"github.com/difyz9/ytb2bili/internal/agentic"
	"github.com/difyz9/ytb2bili/internal/core"
)

// GenerateMetadataTool 生成视频元数据
// 使用 LLM 根据视频内容和字幕生成适合 Bilibili 的标题、简介和标签
type GenerateMetadataTool struct {
	app     *core.AppServer
	videoId string
	llm     agentic.LLMClient
}

// NewGenerateMetadataTool 创建元数据生成工具
func NewGenerateMetadataTool(app *core.AppServer, videoId string, llm agentic.LLMClient) *GenerateMetadataTool {
	return &GenerateMetadataTool{
		app:     app,
		videoId: videoId,
		llm:     llm,
	}
}

func (t *GenerateMetadataTool) Name() string {
	return "generate_metadata"
}

func (t *GenerateMetadataTool) Description() string {
	return `使用 AI 生成或优化视频元数据（标题、描述、标签）。
功能：基于视频内容和字幕，使用 LLM 生成适合 Bilibili 平台的元数据，提高推荐率。
输入格式：视频 ID（字符串）或 JSON:
{
  "video_id": "视频 ID (必需)",
  "source_metadata": "原始元数据 (可选: YouTube 标题、描述)",
  "subtitle_text": "字幕文本摘要 (可选: 用于理解视频内容)",
  "target_lang": "目标语言 (可选: zh-CN, zh-TW, 默认: zh-CN)",
  "style": "风格 (可选: professional, casual, clickbait, 默认: professional)"
}
返回：JSON 格式的元数据
{
  "title": "优化后的视频标题（符合 Bilibili 80 字限制）",
  "description": "视频简介（包含关键信息和标签）",
  "tags": ["标签1", "标签2", "标签3"],
  "category_id": 推荐分区 ID
}

示例输入: {"video_id": "abc123", "target_lang": "zh-CN", "style": "professional"}`
  }
}
返回: 生成的标题、描述和标签`
}

func (t *GenerateMetadataTool) Call(ctx context.Context, input string) (string, error) {

	return "", nil
}

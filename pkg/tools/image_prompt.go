package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zolagz/ytb2bili/pkg/llm"
	"go.uber.org/zap"
)

// GenerateImagePromptTool uses an LLM to expand short descriptions into image-ready prompts.
type GenerateImagePromptTool struct {
	name   string
	desc   string
	llm    LLMClientIface
	logger *zap.Logger
}

// LLMClientIface is the minimal LLM interface needed by GenerateImagePromptTool.
type LLMClientIface interface {
	Chat(ctx context.Context, messages []llm.Message) (string, error)
}

// Name returns the tool name.
func (t *GenerateImagePromptTool) Name() string { return t.name }

// Description returns the tool description.
func (t *GenerateImagePromptTool) Description() string { return t.desc }

// GenerateImagePromptConfig is the optional JSON input schema.
type GenerateImagePromptConfig struct {
	Description string `json:"description"`
	Style       string `json:"style"`
	Mood        string `json:"mood"`
	DetailLevel string `json:"detail_level"`
	Language    string `json:"language"`
}

// NewGenerateImagePromptTool constructs the tool.
func NewGenerateImagePromptTool(llm LLMClientIface, logger *zap.Logger) *GenerateImagePromptTool {
	return &GenerateImagePromptTool{
		name: "generate_image_prompt",
		desc: `使用 AI 将简单的文本描述转换为详细的图片生成提示词。
功能：基于用户的简单描述，使用 LLM 生成适合 AI 图片生成模型（如 DALL-E、Stable Diffusion 等）的详细提示词。
输入格式：文本描述（字符串）或 JSON`,
		llm:    llm,
		logger: logger,
	}
}

// Call executes the tool.
func (t *GenerateImagePromptTool) Call(ctx context.Context, input string) (string, error) {
	t.logger.Info("Generating image prompt", zap.String("input", input))

	cfg := GenerateImagePromptConfig{
		Style:       "digital_art",
		Mood:        "vibrant",
		DetailLevel: "detailed",
		Language:    "en",
	}

	if input != "" && strings.HasPrefix(input, "{") {
		if err := json.Unmarshal([]byte(input), &cfg); err != nil {
			t.logger.Warn("Failed to parse JSON input, treating as plain text", zap.Error(err))
			cfg.Description = input
		}
	} else if input != "" {
		cfg.Description = strings.TrimSpace(input)
	}

	if cfg.Description == "" {
		return "", fmt.Errorf("description cannot be empty")
	}

	if t.llm == nil {
		return "", fmt.Errorf("LLM client not initialized")
	}

	prompt := buildPrompt(cfg)
	messages := []llm.Message{{Role: "user", Content: prompt}}

	resp, err := t.llm.Chat(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("LLM generation failed: %w", err)
	}

	return strings.TrimSpace(resp), nil
}

func buildPrompt(cfg GenerateImagePromptConfig) string {
	var sb strings.Builder
	sb.WriteString("Generate a detailed image generation prompt based on the following info. Respond only with the final prompt.\n")
	sb.WriteString("Description: " + cfg.Description + "\n")
	sb.WriteString("Style: " + cfg.Style + "\n")
	sb.WriteString("Mood: " + cfg.Mood + "\n")
	sb.WriteString("Detail Level: " + cfg.DetailLevel + "\n")
	sb.WriteString("Language: " + cfg.Language + "\n")
	sb.WriteString("Guidelines: Be concise, vivid, include composition, lighting, and medium if helpful.")
	return sb.String()
}

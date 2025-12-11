package agentic

import "context"

// Tool 基于 langchaingo 的工具接口
// 每个工具都是一个独立的能力单元，可以被 Agent 调用
type Tool interface {
	// Name 返回工具的唯一标识符
	Name() string

	// Description 返回工具的自然语言描述，用于 LLM 理解工具的用途
	Description() string

	// Call 执行工具的核心逻辑，input 为自然语言或 JSON 格式的输入
	Call(ctx context.Context, input string) (string, error)
}

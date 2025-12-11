package agentic

import (
	"context"

	"go.uber.org/zap"
)

// Agent LLM 代理，负责根据自然语言任务选择和编排工具
type Agent struct {
	// Name 代理名称
	Name string

	// Tools 可用工具集合
	Tools map[string]Tool

	// LLM 大语言模型客户端
	LLM LLMClient

	// Logger 日志记录器
	Logger *zap.Logger

	// MaxIterations 最大迭代次数（防止无限循环）
	MaxIterations int
}

// LLMClient 大语言模型客户端接口
type LLMClient interface {
	// Chat 对话接口
	Chat(ctx context.Context, messages []Message) (string, error)

	// ChatStream 流式对话接口
	ChatStream(ctx context.Context, messages []Message) (<-chan string, error)
}

// Message LLM 消息
type Message struct {
	Role    string `json:"role"`           // system, user, assistant, tool
	Content string `json:"content"`        // 消息内容
	Name    string `json:"name,omitempty"` // 工具名称（仅用于 tool 角色）
}

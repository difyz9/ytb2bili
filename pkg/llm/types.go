package llm

import "fmt"

// ChatOptions controls per-request generation settings.
type ChatOptions struct {
	Model       string
	Temperature *float32
	MaxTokens   *int
}

// Message 对话消息
type Message struct {
	Role       string     `json:"role"`                   // system, user, assistant, tool
	Content    string     `json:"content"`                // 消息内容
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   // 工具调用（assistant -> tool）
	ToolCallID string     `json:"tool_call_id,omitempty"` // 工具调用 ID（tool -> assistant）
	Name       string     `json:"name,omitempty"`         // 工具名称
}

// ToolCall 工具调用
type ToolCall struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"` // function
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
	Function  *FunctionCall  `json:"function,omitempty"`
}

// FunctionCall 函数调用
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON 字符串
}

// LLMResponse LLM 响应
type LLMResponse struct {
	Content      string     `json:"content"`              // 响应内容
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"` // 工具调用
	FinishReason string     `json:"finish_reason"`        // stop, tool_calls, length, content_filter
	Usage        *UsageInfo `json:"usage,omitempty"`      // Token 使用情况
}

// UsageInfo Token 使用信息
type UsageInfo struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ToolDefinition 工具定义
type ToolDefinition struct {
	Type     string                 `json:"type"` // function
	Function ToolFunctionDefinition `json:"function"`
}

// ToolFunctionDefinition 工具函数定义
type ToolFunctionDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"` // JSON Schema
}

// FailoverReason 失败原因（用于回退决策）
type FailoverReason string

const (
	FailoverAuth      FailoverReason = "auth"       // 认证失败
	FailoverRateLimit FailoverReason = "rate_limit" // 速率限制
	FailoverBilling   FailoverReason = "billing"    // 计费问题
	FailoverTimeout   FailoverReason = "timeout"    // 超时
	FailoverNetwork   FailoverReason = "network"    // 网络错误
	FailoverContext   FailoverReason = "context"    // 上下文长度超限
	FailoverOther     FailoverReason = "other"      // 其他错误
)

// ProviderError Provider 错误
type ProviderError struct {
	Provider string
	Model    string
	Reason   FailoverReason
	Original error
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("provider %s (model %s) failed: %s: %v",
		e.Provider, e.Model, e.Reason, e.Original)
}

// 常用 Vendor 前缀
const (
	VendorOpenAI     = "openai"
	VendorAnthropic  = "anthropic"
	VendorZhipu      = "zhipu"
	VendorDeepSeek   = "deepseek"
	VendorGemini     = "gemini"
	VendorGroq       = "groq"
	VendorMoonshot   = "moonshot"
	VendorQwen       = "qwen"
	VendorOllama     = "ollama"
	VendorOpenRouter = "openrouter"
	VendorCerebras   = "cerebras"
)

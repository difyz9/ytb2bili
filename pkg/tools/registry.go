// Package tools — ToolRegistry 统一管理 eino 工具的注册、注入与执行。
package tools

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// ContextualTool is implemented by tools that support per-request user context injection.
type ContextualTool interface {
	SetUserContext(userID string)
}

// toolErrorHint 工具出错时追加给 LLM 的引导提示，鼓励 LLM 反思并调整策略。
// 参考 nanobot 的 _HINT 设计。
const toolErrorHint = "\n\n[请分析以上错误原因，并尝试不同的方案或参数。]"

// ToolRegistry 统一管理工具的注册与注入，替代手写 if/err 样板。
// 每个工具自动被 hintTool 包装，出错时追加引导提示。
type ToolRegistry struct {
	list []tool.InvokableTool
}

// NewToolRegistry 创建空的 ToolRegistry。
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{}
}

// MustRegister 注册工具。若工具为 nil 或 Info() 返回错误，只记录 Warn 不中断进程。
// 参数 name 仅用于日志，会与 tool.Info().Name 保持一致。
func (r *ToolRegistry) MustRegister(logger *zap.Logger, t tool.InvokableTool, name string) {
	if t == nil {
		logger.Warn("tool is nil, skipped", zap.String("tool", name))
		return
	}
	if _, err := t.Info(context.Background()); err != nil {
		logger.Warn("tool.Info() failed, skipped", zap.String("tool", name), zap.Error(err))
		return
	}
	r.list = append(r.list, &hintTool{t})
	logger.Info("tool registered", zap.String("tool", name))
}

// All 返回所有已注册的工具（转为 BaseTool 切片，兼容 fx group 和 eino）。
func (r *ToolRegistry) All() []tool.BaseTool {
	out := make([]tool.BaseTool, len(r.list))
	for i, t := range r.list {
		out[i] = t
	}
	return out
}

// InjectUserContext 遍历工具列表，为所有实现了 ContextualTool 的工具注入 userID。
// 在每次 Agent.Run 前调用，保证工具用正确的请求级上下文过滤数据。
func InjectUserContext(toolsList []tool.BaseTool, userID string) {
	for _, t := range toolsList {
		switch v := t.(type) {
		case ContextualTool:
			v.SetUserContext(userID)
		case *hintTool:
			if ct, ok := v.inner.(ContextualTool); ok {
				ct.SetUserContext(userID)
			}
		}
	}
}

// hintTool 装饰器：包装任意 InvokableTool，在工具返回错误或错误字符串时自动追加引导提示。
// 这让 LLM 在工具失败后主动反思策略，而非无限重试相同调用。
type hintTool struct {
	inner tool.InvokableTool
}

func (h *hintTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return h.inner.Info(ctx)
}

func (h *hintTool) InvokableRun(ctx context.Context, args string, opts ...tool.Option) (string, error) {
	result, err := h.inner.InvokableRun(ctx, args, opts...)
	if err != nil {
		return "[工具执行失败] " + err.Error() + toolErrorHint, nil
	}
	if looksLikeError(result) {
		result += toolErrorHint
	}
	return result, nil
}

func looksLikeError(s string) bool {
	return strings.HasPrefix(s, "Error") ||
		strings.HasPrefix(s, "错误") ||
		strings.HasPrefix(s, "[") && strings.Contains(s, "] 参数") ||
		strings.HasPrefix(s, "[") && strings.Contains(s, "] 缺少")
}

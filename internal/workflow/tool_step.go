package workflow

// ToolStep 是一个通用包装器，将 pkg/tools 中的任意工具包装为 workflow.Step。
//
// 解决的问题：当前 DownloadVideoStep、ExtractAudioStep 等步骤各自持有工具实例
// 并重复实现相同的参数构建 / 结果解析逻辑，导致两套代码（tools + workflow）维护同一功能。
//
// 基本用法（新步骤可用一个函数完成注册）：
//
//	func NewMyStep(t *tools.MyTool, logger *zap.Logger) Step {
//	    return NewToolStep(
//	        NewBaseStepWithOrder("MyStep", true, 10),
//	        t,
//	        func(vctx *VideoContext) (string, error) {
//	            return fmt.Sprintf(`{"video_url":%q}`, vctx.VideoURL), nil
//	        },
//	        func(vctx *VideoContext, result string) error {
//	            vctx.VideoPath = result
//	            return nil
//	        },
//	    )
//	}
//
// 带可选行为（跳过 / 钩子）：
//
//	return NewToolStep(base, t, argsBuilder, resultApplier,
//	    WithSkipFunc(func(ctx context.Context, vctx *VideoContext) bool {
//	        return vctx.VideoPath != ""
//	    }),
//	    WithOnSuccess(func(ctx context.Context, output any) error {
//	        logger.Info("done"); return nil
//	    }),
//	)
//
// 现有步骤无需立即迁移；新增步骤推荐使用此模式。

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
)

// ToolRunner 是工具执行的最小接口，兼容 tool.BaseTool 以及直接实现 Call 的工具。
type ToolRunner interface {
	InvokableRun(ctx context.Context, args string, opts ...tool.Option) (string, error)
}

// StringCaller 适配只提供 Call(ctx, input string) 的工具。
type StringCaller interface {
	Call(ctx context.Context, input string) (string, error)
}

// StringCallRunner 将单字符串输入的 Call 风格工具适配为 ToolRunner。
// jsonField 约定 JSON 参数中的字符串字段名。
type StringCallRunner struct {
	Tool      StringCaller
	JSONField string
	Name      string
}

func (r StringCallRunner) InvokableRun(ctx context.Context, args string, _ ...tool.Option) (string, error) {
	field := r.JSONField
	if field == "" {
		field = "input"
	}

	var payload map[string]string
	if err := json.Unmarshal([]byte(args), &payload); err != nil {
		return "", fmt.Errorf("unmarshal %s args: %w", runnerName(r.Name), err)
	}

	return r.Tool.Call(ctx, payload[field])
}

func runnerName(name string) string {
	if name == "" {
		return "string call runner"
	}
	return name
}

// ToolStepOption 配置 ToolStep 的可选行为。
type ToolStepOption func(*ToolStep)

// WithSkipFunc 设置跳过条件，使 ToolStep 实现 StepWithSkip。
func WithSkipFunc(fn func(ctx context.Context, vctx *VideoContext) bool) ToolStepOption {
	return func(s *ToolStep) { s.skipFunc = fn }
}

// WithOnSuccess 设置成功回调，使 ToolStep 实现 StepWithHooks。
func WithOnSuccess(fn func(ctx context.Context, output any) error) ToolStepOption {
	return func(s *ToolStep) { s.onSuccess = fn }
}

// WithOnError 设置失败回调，使 ToolStep 实现 StepWithHooks。
func WithOnError(fn func(ctx context.Context, err error) error) ToolStepOption {
	return func(s *ToolStep) { s.onError = fn }
}

// WithRunContext 设置工具执行前的 context 变换逻辑。
func WithRunContext(fn func(ctx context.Context, vctx *VideoContext) (context.Context, error)) ToolStepOption {
	return func(s *ToolStep) { s.runContext = fn }
}

// WithSkipOnError 将工具执行或结果应用错误转为 skipped，而不是 failed。
func WithSkipOnError() ToolStepOption {
	return func(s *ToolStep) { s.skipOnError = true }
}

// ToolStep 将 ToolRunner 包装为 workflow.Step。
type ToolStep struct {
	BaseStep
	runner        ToolRunner
	argsBuilder   func(vctx *VideoContext) (string, error)
	resultApplier func(vctx *VideoContext, result string) error

	skipFunc      func(ctx context.Context, vctx *VideoContext) bool
	runContext    func(ctx context.Context, vctx *VideoContext) (context.Context, error)
	onSuccess     func(ctx context.Context, output any) error
	onError       func(ctx context.Context, err error) error
	skipOnError bool
}

// NewToolStep 创建 ToolStep。
//   - base：步骤元信息（名称、顺序、是否必需）
//   - runner：实际执行工具
//   - argsBuilder：从 VideoContext 构建工具入参 JSON
//   - resultApplier：将工具输出写回 VideoContext（返回 error 会使步骤失败）
//   - opts：可选行为配置
func NewToolStep(
	base BaseStep,
	runner ToolRunner,
	argsBuilder func(vctx *VideoContext) (string, error),
	resultApplier func(vctx *VideoContext, result string) error,
	opts ...ToolStepOption,
) *ToolStep {
	s := &ToolStep{
		BaseStep:      base,
		runner:        runner,
		argsBuilder:   argsBuilder,
		resultApplier: resultApplier,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Execute 实现 workflow.Step 接口。
func (s *ToolStep) Execute(ctx context.Context, input any) (any, error) {
	vctx, err := mustVideoContext(input)
	if err != nil {
		return nil, err
	}

	args, err := s.argsBuilder(vctx)
	if err != nil {
		return nil, fmt.Errorf("step %s: build args failed: %w", s.Name(), err)
	}

	if s.runContext != nil {
		ctx, err = s.runContext(ctx, vctx)
		if err != nil {
			return nil, fmt.Errorf("step %s: prepare context failed: %w", s.Name(), err)
		}
	}

	result, err := s.runner.InvokableRun(ctx, args)
	if err != nil {
		if s.skipOnError {
			return vctx, &StepSkippedError{
				Step:   s.Name(),
				Cause:  fmt.Errorf("step %s: tool run failed: %w", s.Name(), err),
				Output: vctx,
			}
		}
		return nil, fmt.Errorf("step %s: tool run failed: %w", s.Name(), err)
	}

	if err := s.resultApplier(vctx, result); err != nil {
		if s.skipOnError {
			return vctx, &StepSkippedError{
				Step:   s.Name(),
				Cause:  fmt.Errorf("step %s: apply result failed: %w", s.Name(), err),
				Output: vctx,
			}
		}
		return nil, fmt.Errorf("step %s: apply result failed: %w", s.Name(), err)
	}

	return vctx, nil
}

// ShouldSkip 实现 StepWithSkip（仅当 WithSkipFunc 已设置时生效）。
func (s *ToolStep) ShouldSkip(ctx context.Context, input any) bool {
	if s.skipFunc == nil {
		return false
	}
	vctx, ok := input.(*VideoContext)
	if !ok {
		return false
	}
	return s.skipFunc(ctx, vctx)
}

// OnSuccess 实现 StepWithHooks。
func (s *ToolStep) OnSuccess(ctx context.Context, output any) error {
	if s.onSuccess != nil {
		return s.onSuccess(ctx, output)
	}
	return nil
}

// OnError 实现 StepWithHooks。
func (s *ToolStep) OnError(ctx context.Context, err error) error {
	if s.onError != nil {
		return s.onError(ctx, err)
	}
	return nil
}


package workflow

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/fx"
)

// Step 定义任务链中的一个步骤接口
type Step interface {
	// Name 返回步骤名称
	Name() string

	// Execute 执行步骤，接收上下文和输入，返回输出和错误
	Execute(ctx context.Context, input any) (any, error)

	// IsRequired 是否为必需步骤（失败时是否中断链）
	IsRequired() bool

	// Order 返回执行顺序（越小越先执行）
	Order() int
}

// StepWithSkip 支持跳过条件的步骤
type StepWithSkip interface {
	Step
	// ShouldSkip 返回是否应该跳过此步骤
	ShouldSkip(ctx context.Context, input any) bool
}

// StepWithHooks 支持生命周期钩子的步骤
type StepWithHooks interface {
	Step
	// OnSuccess 成功后的回调
	OnSuccess(ctx context.Context, output any) error
	// OnError 失败后的回调
	OnError(ctx context.Context, err error) error
}

// StepSkippedError 表示步骤在执行后因可容忍错误被视为跳过。
// 任务链会继续执行后续步骤，并将该步骤持久化为 skipped。
type StepSkippedError struct {
	Step   string
	Cause  error
	Output any
}

func (e *StepSkippedError) Error() string {
	if e == nil {
		return ""
	}
	if e.Step != "" && e.Cause != nil {
		return fmt.Sprintf("step %s skipped after error: %v", e.Step, e.Cause)
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	if e.Step != "" {
		return fmt.Sprintf("step %s skipped", e.Step)
	}
	return "step skipped"
}

func (e *StepSkippedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func IsStepSkippedError(err error) bool {
	var skipErr *StepSkippedError
	return errors.As(err, &skipErr)
}

// BaseStep 步骤的基础实现
type BaseStep struct {
	name     string
	required bool
	order    int
}

func NewBaseStep(name string, required bool) BaseStep {
	return BaseStep{
		name:     name,
		required: required,
		order:    999, // 默认优先级较低
	}
}

func NewBaseStepWithOrder(name string, required bool, order int) BaseStep {
	return BaseStep{
		name:     name,
		required: required,
		order:    order,
	}
}

func (s BaseStep) Name() string {
	return s.name
}

func (s BaseStep) IsRequired() bool {
	return s.required
}

func (s BaseStep) Order() int {
	return s.order
}

// mustVideoContext 从 input 安全提取 *VideoContext，类型不符时返回明确错误（而非 panic）
func mustVideoContext(input any) (*VideoContext, error) {
	vctx, ok := input.(*VideoContext)
	if !ok || vctx == nil {
		return nil, fmt.Errorf("expected *VideoContext, got %T", input)
	}
	return vctx, nil
}

// AsStep 将任意函数转换为 fx provider 的辅助函数
func AsStep(constructor any) any {
	return fx.Annotate(
		constructor,
		fx.As(new(Step)),
		fx.ResultTags(`group:"steps"`),
	)
}

// StepsIn 用于接收所有步骤的参数结构
type StepsIn struct {
	fx.In
	Steps []Step `group:"steps"`
}

package workflow

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Chain 任务链执行器
type Chain struct {
	name    string
	steps   []Step
	logger  *zap.Logger
	tracker *ProgressTracker // 可选的进度追踪器
}

// WithTracker 设置进度追踪器（链式调用）
func (c *Chain) WithTracker(tracker *ProgressTracker) *Chain {
	c.tracker = tracker
	return c
}

// ChainParams 任务链的依赖参数
type ChainParams struct {
	fx.In
	Name   string `optional:"true"` // 可选的链名称
	Steps  []Step `group:"steps"`   // 自动注入所有步骤
	Logger *zap.Logger
}

// NewChain 创建新的任务链
func NewChain(params ChainParams) *Chain {
	name := params.Name
	if name == "" {
		name = "TaskChain"
	}

	// 按照 Order 对步骤进行排序
	steps := make([]Step, len(params.Steps))
	copy(steps, params.Steps)
	sort.Slice(steps, func(i, j int) bool {
		return steps[i].Order() < steps[j].Order()
	})

	return &Chain{
		name:   name,
		steps:  steps,
		logger: params.Logger,
	}
}

// Result 任务链执行结果
type Result struct {
	Success       bool
	Error         error
	FinalOutput   any
	ExecutedSteps int
	SkippedSteps  int
	FailedSteps   int
	Duration      time.Duration
	StepDetails   map[string]*StepDetail
}

// StepDetail 单个步骤的执行详情
type StepDetail struct {
	Name     string
	Success  bool
	Skipped  bool
	Error    error
	Duration time.Duration
	Output   any
}

// Run 执行任务链
func (c *Chain) Run(ctx context.Context, input any) *Result {
	startTime := time.Now()

	result := &Result{
		Success:     true,
		StepDetails: make(map[string]*StepDetail),
	}

	c.logger.Info("Starting task chain",
		zap.String("chain", c.name),
		zap.Int("steps", len(c.steps)))

	currentInput := input

	for i, step := range c.steps {
		// 每步开始前检查 context 是否已取消
		select {
		case <-ctx.Done():
			result.Success = false
			result.Error = ctx.Err()
			return result
		default:
		}

		detail := c.executeStep(ctx, step, i+1, currentInput)
		result.StepDetails[step.Name()] = detail

		if detail.Skipped {
			result.SkippedSteps++
			continue
		}

		result.ExecutedSteps++

		if !detail.Success {
			result.FailedSteps++

			if step.IsRequired() {
				result.Success = false
				result.Error = fmt.Errorf("required step '%s' failed: %w", step.Name(), detail.Error)
				c.logger.Error("Required step failed, aborting",
					zap.String("step", step.Name()),
					zap.Error(detail.Error))
				break
			}

			c.logger.Warn("Optional step failed, continuing",
				zap.String("step", step.Name()),
				zap.Error(detail.Error))
			continue
		}

		currentInput = detail.Output
	}

	result.FinalOutput = currentInput
	result.Duration = time.Since(startTime)

	c.logger.Info("Task chain completed",
		zap.String("chain", c.name),
		zap.Bool("success", result.Success),
		zap.Int("executed", result.ExecutedSteps),
		zap.Int("skipped", result.SkippedSteps),
		zap.Int("failed", result.FailedSteps),
		zap.Duration("duration", result.Duration))

	return result
}

// executeStep 执行单个步骤
func (c *Chain) executeStep(ctx context.Context, step Step, stepNum int, input any) *StepDetail {
	startTime := time.Now()
	videoID := GetVideoID(ctx)
	if c.tracker != nil {
		ctx = WithProgressTracker(ctx, c.tracker)
	}

	detail := &StepDetail{
		Name:    step.Name(),
		Success: true,
	}

	c.logger.Debug("Executing step",
		zap.Int("step_num", stepNum),
		zap.String("step", step.Name()))

	if shouldSkipForRestartStep(input, step) {
		detail.Skipped = true
		detail.Duration = time.Since(startTime)
		c.logger.Debug("Step skipped before requested restart step", zap.String("step", step.Name()))
		if c.tracker != nil && videoID != "" {
			c.tracker.AfterStep(videoID, step.Name(), "skipped", "")
		}
		return detail
	}

	// 检查是否应该跳过
	if skipStep, ok := step.(StepWithSkip); ok && skipStep.ShouldSkip(ctx, input) {
		detail.Skipped = true
		detail.Duration = time.Since(startTime)
		c.logger.Debug("Step skipped", zap.String("step", step.Name()))
		// 记录跳过状态
		if c.tracker != nil && videoID != "" {
			c.tracker.AfterStep(videoID, step.Name(), "skipped", "")
		}
		return detail
	}

	// 标记步骤为运行中
	if c.tracker != nil && videoID != "" {
		c.tracker.BeforeStep(videoID, step.Name())
	}

	// 执行步骤
	output, err := step.Execute(ctx, input)
	detail.Duration = time.Since(startTime)

	if err != nil {
		var skipErr *StepSkippedError
		if errors.As(err, &skipErr) {
			detail.Skipped = true
			detail.Error = skipErr.Cause
			detail.Output = skipErr.Output

			if c.tracker != nil && videoID != "" {
				c.tracker.AfterStep(videoID, step.Name(), "skipped", "")
			}

			if hookStep, ok := step.(StepWithHooks); ok {
				if hookErr := hookStep.OnError(ctx, skipErr.Cause); hookErr != nil {
					c.logger.Warn("Error hook failed",
						zap.String("step", step.Name()),
						zap.Error(hookErr))
				}
			}

			c.logger.Warn("Step skipped after non-fatal error",
				zap.String("step", step.Name()),
				zap.Error(skipErr.Cause))
			return detail
		}

		detail.Success = false
		detail.Error = err

		// 记录失败状态
		if c.tracker != nil && videoID != "" {
			c.tracker.AfterStep(videoID, step.Name(), "failed", err.Error())
		}

		// 调用错误钩子
		if hookStep, ok := step.(StepWithHooks); ok {
			if hookErr := hookStep.OnError(ctx, err); hookErr != nil {
				c.logger.Warn("Error hook failed",
					zap.String("step", step.Name()),
					zap.Error(hookErr))
			}
		}

		return detail
	}

	detail.Output = output

	// 记录成功状态
	if c.tracker != nil && videoID != "" {
		c.tracker.AfterStep(videoID, step.Name(), "completed", "")
	}

	// 调用成功钩子
	if hookStep, ok := step.(StepWithHooks); ok {
		if hookErr := hookStep.OnSuccess(ctx, output); hookErr != nil {
			c.logger.Warn("Success hook failed",
				zap.String("step", step.Name()),
				zap.Error(hookErr))
		}
	}

	c.logger.Debug("Step completed",
		zap.String("step", step.Name()),
		zap.Duration("duration", detail.Duration))

	return detail
}

func shouldSkipForRestartStep(input any, step Step) bool {
	vctx, ok := input.(*VideoContext)
	if !ok || vctx == nil {
		return false
	}
	restartFromStep := strings.TrimSpace(vctx.RestartFromStep)
	if restartFromStep == "" || vctx.restartStepActivated {
		return false
	}
	if step.Name() == restartFromStep {
		vctx.restartStepActivated = true
		return false
	}
	return true
}

// WithSteps 允许手动设置步骤（用于测试或特殊场景）
func (c *Chain) WithSteps(steps []Step) *Chain {
	c.steps = steps
	return c
}

// clone 深拷贝 Chain（用于单次运行时附加 tracker，不污染原实例）
func (c *Chain) clone() *Chain {
	newChain := *c
	newChain.steps = make([]Step, len(c.steps))
	copy(newChain.steps, c.steps)
	return &newChain
}

// GetSteps 获取所有步骤
func (c *Chain) GetSteps() []Step {
	return c.steps
}

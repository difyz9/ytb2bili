package workflow

import (
	"context"
	"fmt"
	"sort"

	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// NewChainFromSteps 从步骤切片构建 Chain，按 Order 排序。
// 用于替代手动 sort + 构造 Chain{} 的重复逻辑。
func NewChainFromSteps(steps []Step, logger *zap.Logger, name string) *Chain {
	if name == "" {
		name = "TaskChain"
	}
	sorted := make([]Step, len(steps))
	copy(sorted, steps)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Order() < sorted[j].Order()
	})
	return &Chain{
		name:   name,
		steps:  sorted,
		logger: logger,
	}
}

// RunChainWithTracking 封装 tracker 初始化 + chain clone + WithTracker + Run 的通用流程。
// 返回 *VideoContext 或 error。
func RunChainWithTracking(
	ctx context.Context,
	chain *Chain,
	db *gorm.DB,
	logger *zap.Logger,
	videoID string,
	input any,
) (*VideoContext, error) {
	tracker := NewProgressTracker(db, logger)
	if err := tracker.InitSteps(videoID, chain.GetSteps()); err != nil {
		logger.Warn("初始化任务步骤记录失败",
			zap.String("video_id", videoID), zap.Error(err))
	}

	run := chain.clone().WithTracker(tracker)

	result := run.Run(ctx, input)
	if !result.Success {
		return nil, result.Error
	}

	vctx, ok := result.FinalOutput.(*VideoContext)
	if !ok {
		return nil, fmt.Errorf("unexpected output type: %T", result.FinalOutput)
	}
	return vctx, nil
}

// AsStepForGroup 将步骤构造函数注册到指定 fx group。
// 统一替代 AsStep（group:"steps"）、AsDouyinStep（group:"douyin_steps"）、AsBilibiliStep（group:"bilibili_steps"）。
func AsStepForGroup(group string, constructor any) any {
	return fx.Annotate(
		constructor,
		fx.As(new(Step)),
		fx.ResultTags(fmt.Sprintf(`group:"%s"`, group)),
	)
}

// StepProvidersForGroup 将多个步骤构造函数批量注册到指定 fx group。
// 统一替代 asStepProviders / asDouyinStepProviders 等重复辅助函数。
func StepProvidersForGroup(group string, ctors ...any) []fx.Option {
	opts := make([]fx.Option, len(ctors))
	for i, ctor := range ctors {
		opts[i] = fx.Provide(AsStepForGroup(group, ctor))
	}
	return opts
}

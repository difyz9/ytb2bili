package workflow

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// ── ChainPipeline ────────────────────────────────────────────────────────────
// ChainPipeline 将 Chain 的 Step 映射为 Pipeline 的 Stage，
// 实现 pyvideotrans 风格的多阶段队列式流水线。
//
// 每个 Step 对应一个 Stage，Stage 之间通过 channel 传递 PipelineTask。
// 不同 Stage 可以有不同的并发度（Workers 数），
// 从而实现「视频A在翻译的同时，视频B在转写」的并行流水线效果。

// ChainPipeline 包装一个 Chain，将其 Step 转换为 Pipeline 中的 Stage。
type ChainPipeline struct {
	chain      *Chain
	pipeline   *Pipeline
	logger     *zap.Logger
}

// NewChainPipeline 从 Chain 创建 ChainPipeline。
// 每个 Step 映射为一个 Stage，Stage 的并发数默认为 2。
func NewChainPipeline(chain *Chain, logger *zap.Logger) *ChainPipeline {
	steps := chain.GetSteps()
	stages := make([]Stage, 0, len(steps))

	for _, step := range steps {
		step := step // capture
		stage := Stage{
			Name:    StageName(step.Name()),
			Workers: defaultStageWorkers(step.Name()),
			Handler: func(ctx context.Context, task *PipelineTask) error {
				if task.Context == nil {
					return fmt.Errorf("pipeline task %s has nil VideoContext", task.ID)
				}

				stepCtx := ctx
				if task.ID != "" {
					stepCtx = WithVideoID(ctx, task.ID)
				}
				if task.UserID != "" {
					stepCtx = WithUserID(stepCtx, task.UserID)
				}

				output, err := step.Execute(stepCtx, task.Context)
				if err != nil {
					return fmt.Errorf("stage %s failed: %w", step.Name(), err)
				}

				if vctx, ok := output.(*VideoContext); ok {
					task.Context = vctx
				}
				return nil
			},
		}
		stages = append(stages, stage)
	}

	p := NewPipeline(stages, logger)
	return &ChainPipeline{
		chain:    chain,
		pipeline: p,
		logger:   logger,
	}
}

// Submit 提交一个视频处理任务到流水线。
// 返回 eventCh 用于监听进度事件，调用方应在使用完毕后 drain 该 channel。
func (cp *ChainPipeline) Submit(ctx context.Context, task *PipelineTask) (<-chan PipelineEvent, error) {
	return cp.pipeline.Submit(ctx, task)
}

// Start 启动流水线
func (cp *ChainPipeline) Start() {
	cp.pipeline.Start()
}

// Stop 停止流水线
func (cp *ChainPipeline) Stop() {
	cp.pipeline.Stop()
}

// Pipeline 返回内部的 Pipeline 实例
func (cp *ChainPipeline) Pipeline() *Pipeline {
	return cp.pipeline
}

// defaultStageWorkers 根据步骤名称返回默认并发数
func defaultStageWorkers(stepName string) int {
	switch stepName {
	case StepNameInitialize:
		return 4 // 初始化轻量，可高并发
	case StepNameDownloadVideo:
		return 2 // 下载网络密集型
	case StepNameDownloadThumbnail:
		return 4
	case StepNameExtractAudio:
		return 3
	case StepNameTranscribe:
		return 3 // ASR 可多个并行
	case StepNameDeepseekTranslate, StepNameLLMTranslate:
		return 2 // LLM 翻译有限并发
	case StepNameGenerateMetadata:
		return 1
	case StepNameSynthesizeSubtitle:
		return 2 // TTS 并发
	case StepNameSaveDatabase:
		return 2
	case StepNameUploadToBilibili:
		return 1
	default:
		return 2
	}
}

// ── PipelineTask 工厂 ────────────────────────────────────────────────────────

// NewPipelineTask 从 VideoContext 创建 PipelineTask
func NewPipelineTask(videoURL, videoID, userID string, vctx *VideoContext) *PipelineTask {
	if vctx == nil {
		vctx = &VideoContext{}
	}
	return &PipelineTask{
		ID:        videoID,
		VideoURL:  videoURL,
		UserID:    userID,
		Context:   vctx,
		Status:    "pending",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

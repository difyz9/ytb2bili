package workflow

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ── Pipeline Types ───────────────────────────────────────────────────────────

// StageName 流水线阶段名称
type StageName string

const (
	StagePrepare   StageName = "prepare"   // 初始化/解析
	StageDownload  StageName = "download"  // 下载视频
	StageTranscribe StageName = "transcribe" // 语音转文字
	StageTranslate StageName = "translate" // 字幕翻译
	StageMetadata  StageName = "metadata"  // 元数据生成
	StageTTS       StageName = "tts"       // 语音合成
	StageFinalize  StageName = "finalize"  // 保存结果
)

// PipelineTask 在流水线中流动的任务
type PipelineTask struct {
	ID        string       // 任务唯一标识（videoID）
	VideoURL  string       // 视频URL
	Platform  string       // 平台: youtube/douyin
	UserID    string       // 用户ID
	Context   *VideoContext // 视频处理上下文
	Status    string       // 当前状态
	Error     error        // 错误信息
	CreatedAt time.Time
	UpdatedAt time.Time
}

// StageHandler 单个流水线阶段的处理函数
type StageHandler func(ctx context.Context, task *PipelineTask) error

// Stage 流水线阶段定义
type Stage struct {
	Name    StageName
	Handler StageHandler
	Workers int // 该阶段的并发工作数
}

// Pipeline 多阶段队列式流水线
// 类似 pyvideotrans 的 producer-consumer 模式，但用 Go channel + goroutine 实现
type Pipeline struct {
	stages  []Stage
	logger  *zap.Logger
	wg      sync.WaitGroup
	cancel  context.CancelFunc
	tasks   map[string]*PipelineTask
	mu      sync.Mutex
	running bool
}

// PipelineEvent 流水线事件（用于进度通知）
type PipelineEvent struct {
	TaskID    string    `json:"task_id"`
	Stage     StageName `json:"stage"`
	Status    string    `json:"status"` // running / completed / failed / skipped
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// NewPipeline 创建流水线
func NewPipeline(stages []Stage, logger *zap.Logger) *Pipeline {
	return &Pipeline{
		stages: stages,
		logger: logger,
		tasks:  make(map[string]*PipelineTask),
	}
}

// Submit 提交任务到流水线（异步，立即返回）
func (p *Pipeline) Submit(ctx context.Context, task *PipelineTask) (<-chan PipelineEvent, error) {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return nil, fmt.Errorf("pipeline not started")
	}
	if _, exists := p.tasks[task.ID]; exists {
		p.mu.Unlock()
		return nil, fmt.Errorf("task %s already exists in pipeline", task.ID)
	}
	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()
	p.tasks[task.ID] = task
	p.mu.Unlock()

	eventCh := make(chan PipelineEvent, 100)

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer close(eventCh)
		p.runTask(ctx, task, eventCh)
	}()

	return eventCh, nil
}

// runTask 运行单个任务通过所有阶段
func (p *Pipeline) runTask(ctx context.Context, task *PipelineTask, eventCh chan<- PipelineEvent) {
	currentCtx := ctx

	for _, stage := range p.stages {
		// Check cancellation
		select {
		case <-currentCtx.Done():
			task.Status = "cancelled"
			task.Error = currentCtx.Err()
			eventCh <- PipelineEvent{
				TaskID: task.ID, Stage: stage.Name,
				Status: "cancelled", Error: currentCtx.Err().Error(),
				Timestamp: time.Now(),
			}
			return
		default:
		}

		// Emit running event
		task.Status = fmt.Sprintf("stage_%s", stage.Name)
		task.UpdatedAt = time.Now()
		eventCh <- PipelineEvent{
			TaskID: task.ID, Stage: stage.Name, Status: "running",
			Timestamp: time.Now(),
		}

		// Execute stage with timeout
		stageCtx, stageCancel := context.WithTimeout(currentCtx, 30*time.Minute)
		stageErr := stage.Handler(stageCtx, task)
		stageCancel()

		if stageErr != nil {
			task.Status = "failed"
			task.Error = stageErr
			task.UpdatedAt = time.Now()
			eventCh <- PipelineEvent{
				TaskID: task.ID, Stage: stage.Name, Status: "failed",
				Error: stageErr.Error(), Timestamp: time.Now(),
			}
			return
		}

		// Emit completed event
		task.UpdatedAt = time.Now()
		eventCh <- PipelineEvent{
			TaskID: task.ID, Stage: stage.Name, Status: "completed",
			Timestamp: time.Now(),
		}
	}

	// All stages completed
	task.Status = "completed"
	task.UpdatedAt = time.Now()
	eventCh <- PipelineEvent{
		TaskID: task.ID, Stage: "done", Status: "completed",
		Timestamp: time.Now(),
	}
}

// Start 启动流水线（初始化资源）
func (p *Pipeline) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.running = true
	_ = ctx
	p.logger.Info("Pipeline started", zap.Int("stages", len(p.stages)))
}

// Stop 停止流水线（取消所有进行中的任务）
func (p *Pipeline) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
	p.wg.Wait()
	p.running = false
	p.logger.Info("Pipeline stopped")
}

// CancelTask 取消指定任务
func (p *Pipeline) CancelTask(taskID string) bool {
	p.mu.Lock()
	task, exists := p.tasks[taskID]
	p.mu.Unlock()
	if !exists {
		return false
	}
	task.Status = "cancelled"
	return true
}

// TaskStatus 查询任务状态
func (p *Pipeline) TaskStatus(taskID string) *PipelineTask {
	p.mu.Lock()
	defer p.mu.Unlock()
	task, exists := p.tasks[taskID]
	if !exists {
		return nil
	}
	return task
}

// ── Predefined Stages ────────────────────────────────────────────────────────

// NewStandardStages 创建 ytb2bili 标准流水线阶段
// 不同平台（youtube/douyin）可传入不同的 resolve 和 download handler
func NewStandardStages(
	resolveFn StageHandler,
	downloadFn StageHandler,
	transcribeFn StageHandler,
	translateFn StageHandler,
	metadataFn StageHandler,
	ttsFn StageHandler,
	finalizeFn StageHandler,
) []Stage {
	return []Stage{
		{Name: StagePrepare, Handler: resolveFn, Workers: 4},
		{Name: StageDownload, Handler: downloadFn, Workers: 2},
		{Name: StageTranscribe, Handler: transcribeFn, Workers: 3},
		{Name: StageTranslate, Handler: translateFn, Workers: 2},
		{Name: StageMetadata, Handler: metadataFn, Workers: 1},
		{Name: StageTTS, Handler: ttsFn, Workers: 2},
		{Name: StageFinalize, Handler: finalizeFn, Workers: 1},
	}
}

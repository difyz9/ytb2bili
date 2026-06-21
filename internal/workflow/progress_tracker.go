package workflow

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/zolagz/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// contextKey 用于 context.WithValue 的键类型
type contextKey string

const (
	videoIDContextKey contextKey = "workflow_video_id"
	userIDContextKey  contextKey = "workflow_user_id"
	trackerContextKey contextKey = "workflow_progress_tracker"
)

// WithVideoID 将视频 ID 存入 context
func WithVideoID(ctx context.Context, videoID string) context.Context {
	return context.WithValue(ctx, videoIDContextKey, videoID)
}

// GetVideoID 从 context 中取出视频 ID
func GetVideoID(ctx context.Context) string {
	if v, ok := ctx.Value(videoIDContextKey).(string); ok {
		return v
	}
	return ""
}

// WithUserID 将用户 ID 存入 context
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDContextKey, userID)
}

// GetUserID 从 context 中取出用户 ID
func GetUserID(ctx context.Context) string {
	if v, ok := ctx.Value(userIDContextKey).(string); ok {
		return v
	}
	return ""
}

// WithProgressTracker 将 ProgressTracker 存入 context，供步骤上报中间进度。
func WithProgressTracker(ctx context.Context, tracker *ProgressTracker) context.Context {
	if tracker == nil {
		return ctx
	}
	return context.WithValue(ctx, trackerContextKey, tracker)
}

// GetProgressTracker 从 context 中取出 ProgressTracker。
func GetProgressTracker(ctx context.Context) *ProgressTracker {
	if v, ok := ctx.Value(trackerContextKey).(*ProgressTracker); ok {
		return v
	}
	return nil
}

// ProgressTracker 负责将任务步骤进度持久化到数据库
type ProgressTracker struct {
	db             *gorm.DB
	logger         *zap.Logger
	stepStartTimes sync.Map // key: "videoID:stepName" -> time.Time
}

// NewProgressTracker 创建 ProgressTracker 实例
func NewProgressTracker(db *gorm.DB, logger *zap.Logger) *ProgressTracker {
	return &ProgressTracker{db: db, logger: logger}
}

// InitSteps 为指定视频初始化所有任务步骤记录（幂等）
func (t *ProgressTracker) InitSteps(videoID string, steps []Step) error {
	if videoID == "" {
		return nil
	}

	var count int64
	if err := t.db.Model(&model.TaskStep{}).Where("video_id = ?", videoID).Count(&count).Error; err != nil {
		return err
	}

	if count > 0 {
		// 已有记录：只重置尚未完成的步骤，保留 completed/skipped 状态
		return t.db.Model(&model.TaskStep{}).
			Where("video_id = ? AND status NOT IN ?", videoID, []string{model.TaskStepStatusCompleted, model.TaskStepStatusSkipped}).
			Updates(map[string]interface{}{
				"status":           model.TaskStepStatusPending,
				"start_time":       nil,
				"end_time":         nil,
				"duration":         0,
				"progress_percent": 0,
				"progress_text":    "",
				"error_msg":        "",
			}).Error
	}

	for _, step := range steps {
		ts := &model.TaskStep{
			VideoID:         videoID,
			StepName:        step.Name(),
			StepOrder:       step.Order(),
			Status:          model.TaskStepStatusPending,
			ProgressPercent: 0,
			ProgressText:    "",
			CanRetry:        true,
		}
		if err := t.db.Create(ts).Error; err != nil {
			t.logger.Warn("创建任务步骤记录失败",
				zap.String("video_id", videoID),
				zap.String("step", step.Name()),
				zap.Error(err))
		}
	}
	return nil
}

// BeforeStep 将步骤状态标记为 running
func (t *ProgressTracker) BeforeStep(videoID, stepName string) {
	if videoID == "" {
		return
	}
	now := time.Now()
	t.stepStartTimes.Store(videoID+":"+stepName, now)
	if err := t.db.Model(&model.TaskStep{}).
		Where("video_id = ? AND step_name = ?", videoID, stepName).
		Updates(map[string]interface{}{
			"status":           model.TaskStepStatusRunning,
			"start_time":       &now,
			"progress_percent": 0,
			"progress_text":    "执行中",
		}).Error; err != nil {
		t.logger.Warn("更新步骤状态失败(before)",
			zap.String("video_id", videoID),
			zap.String("step", stepName),
			zap.Error(err))
	}
}

// AfterStep 将步骤状态标记为 completed / failed / skipped
func (t *ProgressTracker) AfterStep(videoID, stepName, status, errMsg string) {
	if videoID == "" {
		return
	}
	now := time.Now()
	// 在 Go 侧计算持续时间（毫秒），避免 MySQL 方言依赖
	var durationMs int64
	if v, ok := t.stepStartTimes.LoadAndDelete(videoID + ":" + stepName); ok {
		durationMs = now.Sub(v.(time.Time)).Milliseconds()
	}
	updates := map[string]interface{}{
		"status":   status,
		"end_time": &now,
		"duration": durationMs,
	}
	if status == model.TaskStepStatusCompleted || status == model.TaskStepStatusSkipped {
		updates["progress_percent"] = 100
		updates["progress_text"] = ""
	}
	if status == model.TaskStepStatusFailed {
		updates["progress_text"] = compactProgressText(errMsg)
	}
	if errMsg != "" {
		updates["error_msg"] = errMsg
	}
	if status == model.TaskStepStatusFailed {
		updates["can_retry"] = true
	}

	if err := t.db.Model(&model.TaskStep{}).
		Where("video_id = ? AND step_name = ?", videoID, stepName).
		Updates(updates).Error; err != nil {
		t.logger.Warn("更新步骤状态失败(after)",
			zap.String("video_id", videoID),
			zap.String("step", stepName),
			zap.Error(err))
	}
}

// UpdateStepProgress 持久化步骤的中间进度信息。
func (t *ProgressTracker) UpdateStepProgress(videoID, stepName string, percent int, message string) {
	if videoID == "" {
		return
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	if err := t.db.Model(&model.TaskStep{}).
		Where("video_id = ? AND step_name = ?", videoID, stepName).
		Updates(map[string]interface{}{
			"progress_percent": percent,
			"progress_text":    compactProgressText(message),
		}).Error; err != nil {
		t.logger.Warn("更新步骤进度失败",
			zap.String("video_id", videoID),
			zap.String("step", stepName),
			zap.Error(err))
	}
}

func compactProgressText(message string) string {
	if message == "" {
		return ""
	}
	message = strings.TrimSpace(message)
	if len(message) <= 255 {
		return message
	}
	return message[:252] + "..."
}

// ResetStep 重置步骤为待处理（用于重试）
func (t *ProgressTracker) ResetStep(videoID, stepName string) error {
	return t.db.Model(&model.TaskStep{}).
		Where("video_id = ? AND step_name = ?", videoID, stepName).
		Updates(map[string]interface{}{
			"status":           model.TaskStepStatusPending,
			"start_time":       nil,
			"end_time":         nil,
			"duration":         0,
			"progress_percent": 0,
			"progress_text":    "",
			"error_msg":        "",
		}).Error
}

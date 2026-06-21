package workflow

import (
	"time"

	"github.com/zolagz/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func MarkVideoStopped(db *gorm.DB, logger *zap.Logger, videoID string) {
	if db == nil || videoID == "" {
		return
	}

	now := time.Now()
	if err := db.Model(&model.TaskStep{}).
		Where("video_id = ? AND status = ?", videoID, model.TaskStepStatusRunning).
		Updates(map[string]any{
			"status":           model.TaskStepStatusPending,
			"end_time":         &now,
			"progress_percent": 0,
			"progress_text":    "已停止，可重新开始",
			"error_msg":        "",
		}).Error; err != nil && logger != nil {
		logger.Warn("标记运行中步骤为已停止失败",
			zap.String("video_id", videoID),
			zap.Error(err))
	}
}

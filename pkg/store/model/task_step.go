package model

import "time"

// TaskStep 任务步骤记录
type TaskStep struct {
	BaseModel
	VideoID         string     `gorm:"size:100;index;not null" json:"video_id"` // 关联的视频ID
	StepName        string     `gorm:"size:100;not null" json:"step_name"`      // 步骤名称
	StepOrder       int        `gorm:"not null" json:"step_order"`              // 步骤顺序
	Status          string     `gorm:"size:20;not null" json:"status"`          // 状态: pending/running/completed/failed/skipped
	StartTime       *time.Time `json:"start_time"`                              // 开始时间
	EndTime         *time.Time `json:"end_time"`                                // 结束时间
	Duration        int64      `gorm:"default:0" json:"duration"`               // 执行时长（毫秒）
	ProgressPercent int        `gorm:"default:0" json:"progress_percent"`       // 步骤进度百分比（0-100）
	ProgressText    string     `gorm:"size:255" json:"progress_text"`           // 步骤进度说明
	ErrorMsg        string     `gorm:"type:text" json:"error_msg"`              // 错误信息
	CanRetry        bool       `gorm:"default:true" json:"can_retry"`           // 是否可重试
}

// TableName 指定表名
func (TaskStep) TableName() string {
	return "tb_task_steps"
}

// 任务步骤状态常量
const (
	TaskStepStatusPending   = "pending"   // 待执行
	TaskStepStatusRunning   = "running"   // 执行中
	TaskStepStatusCompleted = "completed" // 已完成
	TaskStepStatusFailed    = "failed"    // 失败
	TaskStepStatusSkipped   = "skipped"   // 跳过
)

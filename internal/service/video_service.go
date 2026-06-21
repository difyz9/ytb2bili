package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/zolagz/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// VideoService 封装视频相关的数据库操作，供 handler 层调用。
type VideoService struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewVideoService(db *gorm.DB, logger *zap.Logger) *VideoService {
	return &VideoService{db: db, logger: logger}
}

func (s *VideoService) GetDB() *gorm.DB { return s.db }

// ── Response types ───────────────────────────────────────────────────────────

type VideoWithStepsWrapper struct {
	model.Video
	TaskSteps []model.TaskStep `json:"task_steps"`
}

type VideoListResponse struct {
	Videos     []VideoWithStepsWrapper `json:"videos"`
	Total      int64                   `json:"total"`
	Page       int                     `json:"page"`
	Size       int                     `json:"size"`
	TotalPages int                     `json:"total_pages"`
}

type TabCounts struct {
	All          int64 `json:"all"`
	Processing   int64 `json:"processing"`
	Completed    int64 `json:"completed"`
	Failed       int64 `json:"failed"`
	BiliUploaded int64 `json:"bili_uploaded"`
}

// ── 查询构建 ─────────────────────────────────────────────────────────────────

func (s *VideoService) BuildBaseQuery(userID, sourceType, tab string) *gorm.DB {
	q := s.db.Model(&model.Video{})
	if userID != "" {
		q = q.Where("user_id = ?", userID)
	}
	if sourceType != "" {
		if sourceType == "manual" {
			q = q.Where("operation_type IN (?)", []string{"manual", "submit"})
		} else {
			q = q.Where("operation_type = ?", sourceType)
		}
	}
	switch tab {
	case "processing":
		q = q.Where("status IN (?)", []string{"001", "002", "pending", "processing"})
	case "completed":
		q = q.Where("status IN (?)", []string{"003", "completed", "processed", "ready", "synced"})
	case "failed":
		q = q.Where("status IN (?)", []string{"004", "failed", model.VideoStatusPaused})
	}
	return q
}

// ── CRUD ─────────────────────────────────────────────────────────────────────

func (s *VideoService) Create(ctx context.Context, video *model.Video) error {
	if video.UserID == "" || video.VideoID == "" {
		return fmt.Errorf("user_id 和 video_id 为必填项")
	}
	return s.db.WithContext(ctx).Create(video).Error
}

func (s *VideoService) GetByPrimaryKey(ctx context.Context, id uint) (*model.Video, error) {
	var video model.Video
	if err := s.db.WithContext(ctx).First(&video, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("视频不存在")
		}
		return nil, fmt.Errorf("查询视频失败: %w", err)
	}
	return &video, nil
}

func (s *VideoService) GetWithSteps(ctx context.Context, id uint) (*model.Video, []model.TaskStep, error) {
	video, err := s.GetByPrimaryKey(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	steps, err := s.ListSteps(ctx, video.VideoID)
	if err != nil {
		return nil, nil, err
	}
	return video, steps, nil
}

func (s *VideoService) GetByVideoID(ctx context.Context, videoID string) (*model.Video, error) {
	var video model.Video
	if err := s.db.WithContext(ctx).Where("video_id = ?", videoID).First(&video).Error; err != nil {
		return nil, err
	}
	return &video, nil
}

func (s *VideoService) CountByTab(ctx context.Context, userID, sourceType string) (*TabCounts, error) {
	countTab := func(tab string) (int64, error) {
		var cnt int64
		if err := s.BuildBaseQuery(userID, sourceType, tab).Count(&cnt).Error; err != nil {
			return 0, err
		}
		return cnt, nil
	}
	var counts TabCounts
	var all int64
	if err := s.BuildBaseQuery(userID, sourceType, "").Count(&all).Error; err != nil {
		return nil, fmt.Errorf("查询视频总数失败: %w", err)
	}
	counts.All = all
	var biliUploaded int64
	if err := s.BuildBaseQuery(userID, sourceType, "").
		Where("bili_bvid != '' AND bili_bvid IS NOT NULL").
		Count(&biliUploaded).Error; err != nil {
		return nil, fmt.Errorf("查询B站上传数失败: %w", err)
	}
	counts.BiliUploaded = biliUploaded
	processing, err := countTab("processing")
	if err != nil {
		return nil, fmt.Errorf("查询处理中视频数失败: %w", err)
	}
	counts.Processing = processing
	completed, err := countTab("completed")
	if err != nil {
		return nil, fmt.Errorf("查询已完成视频数失败: %w", err)
	}
	counts.Completed = completed
	failed, err := countTab("failed")
	if err != nil {
		return nil, fmt.Errorf("查询失败视频数失败: %w", err)
	}
	counts.Failed = failed
	return &counts, nil
}

func (s *VideoService) List(ctx context.Context, userID, sourceType, tab string, page, size int) ([]VideoWithStepsWrapper, int64, int, error) {
	if page < 1 {
		page = 1
	}
	if size <= 0 || size > 200 {
		size = 10
	}
	baseQuery := s.BuildBaseQuery(userID, sourceType, tab)
	var total int64
	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, 0, fmt.Errorf("查询视频总数失败: %w", err)
	}
	var videos []model.Video
	if err := baseQuery.Order("created_at desc").Offset((page - 1) * size).Limit(size).Find(&videos).Error; err != nil {
		return nil, 0, 0, fmt.Errorf("查询视频列表失败: %w", err)
	}
	result := make([]VideoWithStepsWrapper, len(videos))
	for i, v := range videos {
		steps, _ := s.ListSteps(ctx, v.VideoID)
		result[i] = VideoWithStepsWrapper{Video: v, TaskSteps: steps}
	}
	totalPages := int((total + int64(size) - 1) / int64(size))
	if totalPages < 1 {
		totalPages = 1
	}
	return result, total, totalPages, nil
}

func (s *VideoService) Update(ctx context.Context, id uint, patch map[string]interface{}) error {
	delete(patch, "id")
	delete(patch, "created_at")
	delete(patch, "updated_at")
	return s.db.WithContext(ctx).Model(&model.Video{}).Where("id = ?", id).Updates(patch).Error
}

func (s *VideoService) Delete(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&model.Video{}, id).Error
}

// ── 视频处理状态管理 ─────────────────────────────────────────────────────────

func (s *VideoService) UpsertAsProcessing(videoID, url, title, userID, platform,
	preferredResolution, speechVoiceName, taskChainSettingsJSON, playlistID string) {
	var existing model.Video
	err := s.db.Where("video_id = ?", videoID).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		newVideo := &model.Video{
			VideoID:             videoID,
			URL:                 url,
			Title:               title,
			UserID:              userID,
			Platform:            platform,
			Status:              model.VideoStatusProcessing,
			PreferredResolution: normRes(preferredResolution),
			SpeechVoiceName:     strings.TrimSpace(speechVoiceName),
			TaskChainSettings:   taskChainSettingsJSON,
			PlaylistID:          strings.TrimSpace(playlistID),
			OperationType:       "manual",
		}
		if err := s.db.Create(newVideo).Error; err != nil {
			s.logger.Warn("创建视频记录失败", zap.String("video_id", videoID), zap.Error(err))
		}
	} else if err == nil {
		updates := map[string]interface{}{
			"platform": platform, "status": model.VideoStatusProcessing,
			"operation_type": "manual", "preferred_resolution": normRes(preferredResolution),
			"speech_voice_name": strings.TrimSpace(speechVoiceName),
			"task_chain_settings": taskChainSettingsJSON, "playlist_id": strings.TrimSpace(playlistID),
		}
		if strings.TrimSpace(userID) != "" {
			updates["user_id"] = userID
		}
		if err := s.db.Model(&existing).Updates(updates).Error; err != nil {
			s.logger.Warn("更新视频状态失败", zap.String("video_id", videoID), zap.Error(err))
		}
	}
}

func (s *VideoService) MarkStatus(ctx context.Context, videoID, status string) {
	s.db.WithContext(ctx).Model(&model.Video{}).Where("video_id = ?", videoID).Update("status", status)
}

func (s *VideoService) UpdateProcessingResult(ctx context.Context, videoID string, updates map[string]interface{}) {
	if err := s.db.WithContext(ctx).Model(&model.Video{}).Where("video_id = ?", videoID).Updates(updates).Error; err != nil {
		s.logger.Warn("更新视频处理结果失败", zap.String("video_id", videoID), zap.Error(err))
	}
}

func (s *VideoService) PersistResumeOverrides(videoID, resolution, voiceName, chainSettingsJSON string) {
	if strings.TrimSpace(videoID) == "" {
		return
	}
	s.db.Model(&model.Video{}).Where("video_id = ?", videoID).Updates(map[string]interface{}{
		"preferred_resolution": normRes(resolution),
		"speech_voice_name":    strings.TrimSpace(voiceName),
		"task_chain_settings":  chainSettingsJSON,
	})
}

// ── 任务步骤管理 ─────────────────────────────────────────────────────────────

func (s *VideoService) ListSteps(ctx context.Context, videoID string) ([]model.TaskStep, error) {
	var steps []model.TaskStep
	if err := s.db.WithContext(ctx).Where("video_id = ?", videoID).Order("step_order asc").Find(&steps).Error; err != nil {
		return nil, err
	}
	return steps, nil
}

func (s *VideoService) ResetStepsFrom(ctx context.Context, videoID, stepName string) error {
	trimmed := strings.TrimSpace(stepName)
	if trimmed == "" {
		return nil
	}
	var target model.TaskStep
	if err := s.db.WithContext(ctx).Where("video_id = ? AND step_name = ?", videoID, trimmed).First(&target).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("步骤 %q 不存在", trimmed)
		}
		return err
	}
	return s.db.WithContext(ctx).Model(&model.TaskStep{}).
		Where("video_id = ? AND step_order >= ?", videoID, target.StepOrder).
		Updates(map[string]any{
			"status": model.TaskStepStatusPending, "start_time": nil, "end_time": nil,
			"duration": 0, "error_msg": "", "progress_percent": 0, "progress_text": "",
		}).Error
}

func (s *VideoService) MarkStepsStopping(ctx context.Context, videoID string) {
	s.db.WithContext(ctx).Model(&model.TaskStep{}).
		Where("video_id = ? AND status = ?", videoID, model.TaskStepStatusRunning).
		Updates(map[string]any{"progress_text": "停止中"})
}

// ── 辅助 ─────────────────────────────────────────────────────────────────────

func normRes(resolution string) string {
	switch strings.TrimSpace(resolution) {
	case "best", "720p", "1080p", "1440p", "2160p", "4k":
		return strings.TrimSpace(resolution)
	default:
		return "best"
	}
}

func ParsePageSize(pageStr, sizeStr string, defaultSize int) (page, size int) {
	page, _ = strconv.Atoi(pageStr)
	size, _ = strconv.Atoi(sizeStr)
	if page < 1 {
		page = 1
	}
	if size <= 0 || size > 200 {
		size = defaultSize
		if size <= 0 {
			size = 10
		}
	}
	return
}

package bilibili

import (
	"fmt"

	"github.com/zolagz/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
)

// UploadResult captures the persisted outcome of a successful Bilibili upload.
type UploadResult struct {
	BiliBVID    string
	BiliAID     int64
	Title       string
	Description string
	Tags        string
}

// PersistUploadResult updates the video record with the upload outcome.
func (s *Service) PersistUploadResult(video *model.Video, result UploadResult) error {
	if video == nil {
		return fmt.Errorf("视频不存在")
	}

	updates := map[string]interface{}{
		"bili_bvid":              result.BiliBVID,
		"bili_aid":               result.BiliAID,
		"bili_subtitle_uploaded": false,
	}
	if result.Title != "" {
		updates["generated_title"] = result.Title
	}
	if result.Description != "" {
		updates["generated_desc"] = result.Description
	}
	if result.Tags != "" {
		updates["generated_tags"] = result.Tags
	}

	if err := s.db.Model(video).Updates(updates).Error; err != nil {
		return fmt.Errorf("更新B站上传结果失败: %w", err)
	}

	video.BiliBVID = result.BiliBVID
	video.BiliAID = result.BiliAID
	video.BiliSubtitleUploaded = false

	if _, err := s.syncSubtitleUploadStates(video); err != nil {
		return fmt.Errorf("同步字幕上传状态失败: %w", err)
	}

	s.logger.Info("上传B站结果已落库",
		zap.String("video_id", video.VideoID),
		zap.String("bvid", result.BiliBVID),
		zap.Int64("aid", result.BiliAID),
		zap.Bool("subtitle_pending", !video.BiliSubtitleUploaded))
	return nil
}

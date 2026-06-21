package service

import (
	"context"
	"fmt"
	"time"

	"github.com/zolagz/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// YouTubeService 封装 YouTube 订阅/Feed 相关的数据库操作。
// 对应 youtube_handler.go 中的 DB 操作。
type YouTubeService struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewYouTubeService(db *gorm.DB, logger *zap.Logger) *YouTubeService {
	return &YouTubeService{db: db, logger: logger}
}

func (s *YouTubeService) GetDB() *gorm.DB { return s.db }

// ── 订阅管理 ─────────────────────────────────────────────────────────────────

// GetUserSubscriptions 查询用户的 YouTube 订阅列表
func (s *YouTubeService) GetUserSubscriptions(ctx context.Context, userID string) ([]model.TbSubscription, error) {
	var subs []model.TbSubscription
	if err := s.db.WithContext(ctx).Model(&model.TbSubscription{}).
		Where("user_id = ? AND platform = 'youtube' AND status = 'active'", userID).
		Find(&subs).Error; err != nil {
		return nil, fmt.Errorf("query subscriptions: %w", err)
	}
	return subs, nil
}

// GetUserSubscriptionsWithVideos 查询用户的订阅列表（含最新视频）
func (s *YouTubeService) GetUserSubscriptionsWithVideos(ctx context.Context, userID string, limit int) ([]model.TbSubscription, error) {
	subs, err := s.GetUserSubscriptions(ctx, userID)
	if err != nil || len(subs) == 0 {
		return subs, err
	}
	if limit <= 0 {
		limit = 12
	}
	// 查最新视频的逻辑保留在原 handler 中（涉及 YouTube API）
	return subs, nil
}

// GetLatestVideos 查询指定频道的最新视频
func (s *YouTubeService) GetLatestVideos(ctx context.Context, userID string, source string, limit int) ([]model.Video, error) {
	query := s.db.WithContext(ctx).Model(&model.Video{}).
		Where("user_id = ?", userID).
		Order("published_at DESC, created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if source == "subscription" {
		subs, err := s.GetUserSubscriptions(ctx, userID)
		if err != nil || len(subs) == 0 {
			return nil, err
		}
		channelIDs := make([]string, len(subs))
		for i, sub := range subs {
			channelIDs[i] = sub.ChannelID
		}
		query = query.Where("channel_id IN ?", channelIDs)
	}
	var videos []model.Video
	if err := query.Find(&videos).Error; err != nil {
		return nil, fmt.Errorf("query latest videos: %w", err)
	}
	return videos, nil
}

// GetUserTbSubscriptions 查询用户所有订阅
func (s *YouTubeService) GetUserTbSubscriptions(ctx context.Context, userID string, page, pageSize int) ([]model.TbSubscription, int64, error) {
	query := s.db.WithContext(ctx).Model(&model.TbSubscription{}).Where("user_id = ?", userID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var subs []model.TbSubscription
	if err := query.Offset((page - 1) * pageSize).Limit(pageSize).Find(&subs).Error; err != nil {
		return nil, 0, err
	}
	return subs, total, nil
}

// UpdateSubscriptionStatus 更新订阅状态
func (s *YouTubeService) UpdateSubscriptionStatus(ctx context.Context, id uint, status string) error {
	return s.db.WithContext(ctx).Model(&model.TbSubscription{}).
		Where("id = ?", id).Update("status", status).Error
}

// MergeSubscriptions 合并订阅列表（删除旧订阅，批量插入新订阅）
func (s *YouTubeService) MergeSubscriptions(ctx context.Context, userID string, subs []model.TbSubscription) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ? AND platform = 'youtube'", userID).
			Delete(&model.TbSubscription{}).Error; err != nil {
			return err
		}
		if len(subs) > 0 {
			if err := tx.CreateInBatches(subs, 50).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ── Feed 视频管理 ────────────────────────────────────────────────────────────

// GetFeedVideo 根据 videoID 查询 feed 视频
func (s *YouTubeService) GetFeedVideo(ctx context.Context, videoID string) (*model.Video, error) {
	var video model.Video
	err := s.db.WithContext(ctx).Where("video_id = ?", videoID).First(&video).Error
	if err != nil {
		return nil, err
	}
	return &video, nil
}

// UpsertFeedVideo 创建或更新 feed 视频记录
func (s *YouTubeService) UpsertFeedVideo(ctx context.Context, video *model.Video) error {
	var existing model.Video
	err := s.db.WithContext(ctx).Where("video_id = ?", video.VideoID).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return s.db.WithContext(ctx).Create(video).Error
	}
	if err != nil {
		return err
	}
	video.ID = existing.ID
	video.CreatedAt = existing.CreatedAt
	video.UpdatedAt = time.Now()
	return s.db.WithContext(ctx).Save(video).Error
}

// UpdateFeedVideo 更新 feed 视频字段
func (s *YouTubeService) UpdateFeedVideo(ctx context.Context, videoID string, updates map[string]interface{}) error {
	return s.db.WithContext(ctx).Model(&model.Video{}).
		Where("video_id = ?", videoID).Updates(updates).Error
}

// DeleteFeedVideo 删除 feed 视频
func (s *YouTubeService) DeleteFeedVideo(ctx context.Context, videoID string) error {
	return s.db.WithContext(ctx).Where("video_id = ?", videoID).Delete(&model.Video{}).Error
}

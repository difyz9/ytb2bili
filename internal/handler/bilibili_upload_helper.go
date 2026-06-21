package handler

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/zolagz/ytb2bili/internal/analytics"
	"github.com/zolagz/ytb2bili/internal/workflow"
	bili "github.com/zolagz/ytb2bili/pkg/bilibili"
	"github.com/zolagz/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
)

func UploadVideoToBilibili(ctx context.Context, logger *zap.Logger, biliService *bili.Service, biliChain *workflow.BilibiliChain, analyticsClient *analytics.Client, userID string, video *model.Video, overrides *workflow.BilibiliSubmissionOverrides) (*workflow.BilibiliContext, error) {
	if biliChain == nil {
		return nil, fmt.Errorf("B站上传服务未配置")
	}
	if biliService == nil {
		return nil, fmt.Errorf("B站模块未配置")
	}
	if video == nil {
		return nil, fmt.Errorf("视频不存在")
	}
	if video.VideoPath == "" {
		return nil, fmt.Errorf("视频文件路径为空，无法上传")
	}
	if userID == "" {
		userID = video.UserID
	}
	if userID == "" {
		return nil, fmt.Errorf("缺少用户ID")
	}

	result, err := biliChain.RunFromVideoPath(ctx, userID, video.VideoPath, video.URL, overrides)
	if err != nil {
		return nil, err
	}

	if err := biliService.PersistUploadResult(video, bili.UploadResult{
		BiliBVID:    result.BiliBVID,
		BiliAID:     result.BiliAID,
		Title:       result.Title,
		Description: result.Description,
		Tags:        result.Tags,
	}); err != nil {
		return nil, err
	}

	logger.Info("上传B站成功",
		zap.String("video_id", video.VideoID),
		zap.String("bvid", result.BiliBVID),
		zap.Int64("aid", result.BiliAID),
	)
	trackBilibiliUploadSuccess(analyticsClient, logger, userID, video, result)
	return result, nil
}

func uploadVideoToBilibili(ctx context.Context, logger *zap.Logger, biliService *bili.Service, biliChain *workflow.BilibiliChain, analyticsClient *analytics.Client, userID string, video *model.Video, overrides *workflow.BilibiliSubmissionOverrides) (*workflow.BilibiliContext, error) {
	return UploadVideoToBilibili(ctx, logger, biliService, biliChain, analyticsClient, userID, video, overrides)
}

func trackBilibiliUploadSuccess(analyticsClient *analytics.Client, logger *zap.Logger, userID string, video *model.Video, result *workflow.BilibiliContext) {
	if analyticsClient == nil || video == nil || result == nil {
		return
	}
	go func() {
		properties := map[string]interface{}{
			"user_id":           userID,
			"video_id":          video.VideoID,
			"source_platform":   video.Platform,
			"source_url":        video.URL,
			"title":             fallbackUploadString(result.Title, video.Title),
			"bvid":              result.BiliBVID,
			"aid":               result.BiliAID,
			"bili_video_url":    "https://www.bilibili.com/video/" + result.BiliBVID,
			"resource_type":     "video",
			"resource_filename": filepath.Base(video.VideoPath),
			"thumbnail":         video.Thumbnail,
			"upload_time":       time.Now().Unix(),
		}
		analyticsClient.TrackBilibiliVideoUploadSuccess(properties)
		// logger.Info("B站上传成功事件已发送到Analytics客户端",
		// 	zap.String("user_id", userID),
		// 	zap.String("video_id", video.VideoID),
		// 	zap.String("bvid", result.BiliBVID))
	}()
}

func fallbackUploadString(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

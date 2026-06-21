package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zolagz/ytb2bili/pkg/store/model"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ============================================================================
// 步骤 6: 保存到数据库（可选）
// ============================================================================

type SaveDatabaseStep struct {
	BaseStep
	db     *gorm.DB
	logger *zap.Logger
}

type SaveDatabaseStepParams struct {
	fx.In
	DB     *gorm.DB
	Logger *zap.Logger
}

func NewSaveDatabaseStep(params SaveDatabaseStepParams) *SaveDatabaseStep {
	return &SaveDatabaseStep{
		BaseStep: NewBaseStepWithOrder(StepNameSaveDatabase, true, 9), // 保存应在水印等最终处理之后
		db:       params.DB,
		logger:   params.Logger,
	}
}

func (s *SaveDatabaseStep) Execute(ctx context.Context, input any) (any, error) {
	vctx, err := mustVideoContext(input)
	if err != nil {
		return nil, err
	}

	// 使用统一的 Video 模型。
	var video model.Video
	err = s.db.WithContext(ctx).Where("video_id = ?", vctx.VideoID).First(&video).Error

	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("query video failed: %w", err)
	}

	updates := map[string]interface{}{
		"video_path": vctx.VideoPath,
		"thumbnail":  vctx.ThumbnailPath,
		"status":     "ready",
	}
	if strings.TrimSpace(vctx.VideoPath) != "" {
		info, statErr := os.Stat(vctx.VideoPath)
		if statErr != nil {
			return nil, fmt.Errorf("stat video file failed: %w", statErr)
		}
		updates["video_size_bytes"] = info.Size()
	}
	if vctx.Platform != "" {
		updates["platform"] = vctx.Platform
	}

	if vctx.Transcript != nil {
		srtPath := filepath.Join(filepath.Dir(vctx.VideoPath), vctx.VideoID+".srt")
		updates["subtitle_path"] = srtPath
	}

	if video.ID == 0 {
		// 创建新视频记录
		video = model.Video{
			UserID:         strings.TrimSpace(vctx.UserID),
			VideoID:        vctx.VideoID,
			Platform:       vctx.Platform,
			URL:            vctx.VideoURL,
			VideoPath:      vctx.VideoPath,
			VideoSizeBytes: updates["video_size_bytes"].(int64),
			Thumbnail:      vctx.ThumbnailPath,
			Status:         "ready",
		}
		if srt, ok := updates["subtitle_path"].(string); ok {
			video.SubtitlePath = srt
		}
		return vctx, s.db.WithContext(ctx).Create(&video).Error
	}

	return vctx, s.db.WithContext(ctx).Model(&video).Updates(updates).Error
}

func (s *SaveDatabaseStep) OnSuccess(ctx context.Context, output any) error {
	vctx := output.(*VideoContext)
	s.logger.Info("Results saved to database", zap.String("video_id", vctx.VideoID))
	return nil
}

func (s *SaveDatabaseStep) OnError(ctx context.Context, err error) error {
	s.logger.Warn("Failed to save to database", zap.Error(err))
	return nil
}

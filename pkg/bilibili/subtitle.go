package bilibili

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	bilisdk "github.com/difyz9/bilibili-go-sdk/bilibili"
	"github.com/zolagz/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// SubtitleCandidate describes a local subtitle file and its target language.
type SubtitleCandidate struct {
	Filename string
	Language string
}

// HasPendingLocalSubtitles reports whether the video still has local subtitle files to upload.
func HasPendingLocalSubtitles(video model.Video) bool {
	for _, path := range existingSubtitlePaths(video) {
		if strings.TrimSpace(path) != "" {
			return true
		}
	}
	return false
}

// UploadApprovedVideoSubtitles checks review status and uploads matching subtitle files.
func (s *Service) UploadApprovedVideoSubtitles(video model.Video) error {
	logger := s.logger.With(
		zap.String("video_id", video.VideoID),
		zap.String("bili_bvid", video.BiliBVID),
	)

	records, err := s.syncSubtitleUploadStates(&video)
	if err != nil {
		logger.Error("同步字幕上传状态失败", zap.Error(err))
		return err
	}
	pendingRecords := filterPendingSubtitleUploads(records)
	if len(pendingRecords) == 0 {
		logger.Debug("字幕状态已全部完成，无需重复上传")
		return nil
	}

	account, err := s.resolveVideoAccount(logger, video)
	if err != nil || account == nil {
		logger.Warn("未找到B站账号，跳过字幕上传", zap.String("user_id", video.UserID), zap.Error(err))
		return err
	}

	loginInfo, err := s.loginInfoFromAccount(account)
	if err != nil {
		logger.Error("解析B站登录信息失败（请重新绑定账号）", zap.Error(err))
		return err
	}

	biliClient := bilisdk.NewClient()
	reviewStatus, err := biliClient.GetVideoReviewStatus(video.BiliBVID, loginInfo.GetCookieString())
	if err != nil {
		logger.Warn("获取B站审核状态失败，稍后重试", zap.Error(err))
		return err
	}
	if !reviewStatus.Passed {
		logger.Info("视频尚未通过审核，跳过字幕上传", zap.String("state_desc", reviewStatus.StateDesc))
		return nil
	}

	uploader := bilisdk.NewSubtitleUploader(biliClient, loginInfo)
	videoInfo, err := uploader.GetVideoInfo(video.BiliBVID)
	if err != nil {
		logger.Warn("获取视频字幕信息失败，稍后重试", zap.Error(err))
		return err
	}
	uploadedCount := 0

	for _, record := range pendingRecords {
		subtitleData, err := bilisdk.LoadSRTAsBCC(record.FilePath)
		if err != nil {
			if updateErr := s.markSubtitleUploadFailed(record.ID, err); updateErr != nil {
				logger.Error("更新字幕上传失败状态失败", zap.Uint("subtitle_upload_id", record.ID), zap.Error(updateErr))
			}
			logger.Error("解析字幕文件失败",
				zap.String("file", record.FileName),
				zap.Error(err))
			continue
		}

		acceptedLanguage, err := saveSubtitleDraft(uploader, video.BiliBVID, videoInfo.CID, subtitleData, record.Language)
		if err != nil {
			if updateErr := s.markSubtitleUploadFailed(record.ID, err); updateErr != nil {
				logger.Error("更新字幕上传失败状态失败", zap.Uint("subtitle_upload_id", record.ID), zap.Error(updateErr))
			}
			logger.Error("上传字幕失败",
				zap.String("file", record.FileName),
				zap.String("lang", record.Language),
				zap.Error(err))
			continue
		}

		if err := s.markSubtitleUploadSucceeded(record.ID); err != nil {
			logger.Error("更新字幕上传成功状态失败", zap.Uint("subtitle_upload_id", record.ID), zap.Error(err))
			return err
		}

		logger.Info("字幕上传成功",
			zap.String("file", record.FileName),
			zap.String("lang", acceptedLanguage))
		uploadedCount++
	}

	if err := s.refreshVideoSubtitleAggregateStatus(&video); err != nil {
		logger.Error("刷新视频字幕汇总状态失败", zap.Error(err))
		return err
	}

	if uploadedCount > 0 {
		logger.Info("字幕上传完成",
			zap.Int("uploaded", uploadedCount),
			zap.String("bili_url", fmt.Sprintf("https://www.bilibili.com/video/%s", video.BiliBVID)))
		return nil
	}

	logger.Info("当前没有成功上传新的字幕文件，保留待重试状态")
	return nil
}

func saveSubtitleDraft(uploader *bilisdk.SubtitleUploader, bvid string, cid int64, subtitle *bilisdk.BCCSubtitle, language string) (string, error) {
	apiLanguage := normalizeSubtitleUploadLanguage(language)
	if err := uploader.SaveSubtitleDraft(bvid, cid, subtitle, apiLanguage); err != nil {
		return "", err
	}
	return apiLanguage, nil
}

func normalizeSubtitleUploadLanguage(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "zh", "zh-cn", "zh-hans", "cmn", "cmn-hans":
		return "zh"
	case "en", "en-us":
		return "en"
	default:
		return strings.TrimSpace(language)
	}
}

func (s *Service) resolveVideoAccount(logger *zap.Logger, video model.Video) (*model.AccountBinding, error) {
	if video.UserID != "" {
		return s.GetPrimaryBiliAccountWithFile(video.UserID)
	}

	logger.Warn("视频 user_id 为空，尝试加载任意可用B站账号")
	return s.GetAnyActiveBiliAccountWithFile()
}

// BuildSubtitleCandidates returns de-duplicated subtitle filenames in priority order.
func BuildSubtitleCandidates(video model.Video) []SubtitleCandidate {
	trimmedVideoID := strings.TrimSpace(video.VideoID)
	if trimmedVideoID == "" {
		return nil
	}

	return []SubtitleCandidate{
		{Filename: trimmedVideoID + ".zh.srt", Language: model.BiliSubtitleLanguageZh},
		{Filename: trimmedVideoID + ".en.srt", Language: model.BiliSubtitleLanguageEn},
	}
}

func existingSubtitlePaths(video model.Video) []string {
	pathSet := make(map[string]struct{})
	var paths []string

	appendPath := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		if _, err := os.Stat(path); err != nil {
			return
		}
		if _, exists := pathSet[path]; exists {
			return
		}
		pathSet[path] = struct{}{}
		paths = append(paths, path)
	}

	appendPath(video.SubtitlePath)

	videoDir := filepath.Dir(video.VideoPath)
	for _, candidate := range BuildSubtitleCandidates(video) {
		appendPath(filepath.Join(videoDir, candidate.Filename))
	}

	return paths
}

func filterPendingSubtitleUploads(records []model.BiliSubtitleUpload) []model.BiliSubtitleUpload {
	pending := make([]model.BiliSubtitleUpload, 0, len(records))
	for _, record := range records {
		if record.Status == model.BiliSubtitleStatusPending || record.Status == model.BiliSubtitleStatusFailed {
			pending = append(pending, record)
		}
	}
	return pending
}

func (s *Service) syncSubtitleUploadStates(video *model.Video) ([]model.BiliSubtitleUpload, error) {
	if video == nil {
		return nil, fmt.Errorf("video is nil")
	}
	if strings.TrimSpace(video.VideoID) == "" {
		return nil, fmt.Errorf("video_id is empty")
	}
	if strings.TrimSpace(video.BiliBVID) == "" {
		return nil, fmt.Errorf("bili_bvid is empty")
	}

	videoDir := filepath.Dir(video.VideoPath)
	now := time.Now()
	candidates := BuildSubtitleCandidates(*video)
	records := make([]model.BiliSubtitleUpload, 0, len(candidates))

	err := s.db.Transaction(func(tx *gorm.DB) error {
		for _, candidate := range candidates {
			filePath := filepath.Join(videoDir, candidate.Filename)
			nextStatus := model.BiliSubtitleStatusMissing
			if _, statErr := os.Stat(filePath); statErr == nil {
				nextStatus = model.BiliSubtitleStatusPending
			}

			record := model.BiliSubtitleUpload{}
			err := tx.Where("video_id = ? AND language = ?", video.VideoID, candidate.Language).First(&record).Error
			if err != nil && err != gorm.ErrRecordNotFound {
				return err
			}

			if err == gorm.ErrRecordNotFound {
				record = model.BiliSubtitleUpload{
					VideoID:      video.VideoID,
					UserID:       video.UserID,
					BiliBVID:     video.BiliBVID,
					Language:     candidate.Language,
					FileName:     candidate.Filename,
					FilePath:     filePath,
					Status:       nextStatus,
					LastCheckedAt: &now,
				}
				if nextStatus == model.BiliSubtitleStatusUploaded {
					record.UploadedAt = &now
				}
				if createErr := tx.Create(&record).Error; createErr != nil {
					return createErr
				}
				records = append(records, record)
				continue
			}

			record.UserID = video.UserID
			record.BiliBVID = video.BiliBVID
			record.FileName = candidate.Filename
			record.FilePath = filePath
			record.LastCheckedAt = &now
			if record.Status != model.BiliSubtitleStatusUploaded {
				record.Status = nextStatus
				if nextStatus != model.BiliSubtitleStatusFailed {
					record.LastError = ""
				}
			}
			if saveErr := tx.Save(&record).Error; saveErr != nil {
				return saveErr
			}
			records = append(records, record)
		}

		return s.refreshVideoSubtitleAggregateStatusTx(tx, video)
	})
	if err != nil {
		return nil, err
	}

	return records, nil
}

func (s *Service) markSubtitleUploadSucceeded(recordID uint) error {
	now := time.Now()
	return s.db.Model(&model.BiliSubtitleUpload{}).
		Where("id = ?", recordID).
		Updates(map[string]any{
			"status":          model.BiliSubtitleStatusUploaded,
			"last_error":      "",
			"uploaded_at":     &now,
			"last_checked_at": &now,
		}).Error
}

func (s *Service) markSubtitleUploadFailed(recordID uint, cause error) error {
	now := time.Now()
	message := "字幕上传失败"
	if cause != nil {
		message = cause.Error()
	}
	return s.db.Model(&model.BiliSubtitleUpload{}).
		Where("id = ?", recordID).
		Updates(map[string]any{
			"status":          model.BiliSubtitleStatusFailed,
			"last_error":      message,
			"attempt_count":   gorm.Expr("attempt_count + 1"),
			"last_checked_at": &now,
		}).Error
}

func (s *Service) refreshVideoSubtitleAggregateStatus(video *model.Video) error {
	if video == nil {
		return fmt.Errorf("video is nil")
	}
	return s.refreshVideoSubtitleAggregateStatusTx(s.db, video)
}

func (s *Service) refreshVideoSubtitleAggregateStatusTx(db *gorm.DB, video *model.Video) error {
	var records []model.BiliSubtitleUpload
	if err := db.Where("video_id = ?", video.VideoID).Find(&records).Error; err != nil {
		return err
	}

	uploaded := len(records) == len(BuildSubtitleCandidates(*video)) && len(records) > 0
	for _, record := range records {
		if record.Status != model.BiliSubtitleStatusUploaded {
			uploaded = false
			break
		}
	}

	if err := db.Model(&model.Video{}).
		Where("id = ?", video.ID).
		Update("bili_subtitle_uploaded", uploaded).Error; err != nil {
		return err
	}
	video.BiliSubtitleUploaded = uploaded
	return nil
}

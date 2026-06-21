package bilibili

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	bilisdk "github.com/difyz9/bilibili-go-sdk/bilibili"
	"github.com/zolagz/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
)

// UploadSubmission captures the metadata required for a Bilibili submission.
type UploadSubmission struct {
	AccountID uint
	Copyright int
	Source    string
	Title     string
	Desc      string
	Tag       string
	Tid       int
	Cover     string
}

// UploadOutcome captures the essential result of a successful upload.
type UploadOutcome struct {
	BVID       string
	AID        int64
	Filename   string
	VideoTitle string
}

type coverUploader interface {
	UploadCover(imagePath string) (string, error)
	UploadCoverFromURL(imageURL string) (string, error)
}

// UploadVideoForUser uploads a local file and submits it as a Bilibili稿件.
func (s *Service) UploadVideoForUser(userID string, videoPath string, submission *UploadSubmission) (*UploadOutcome, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, fmt.Errorf("未找到用户ID，请先登录")
	}
	if strings.TrimSpace(videoPath) == "" {
		return nil, fmt.Errorf("未找到视频文件")
	}
	if submission == nil {
		return nil, fmt.Errorf("投稿信息不能为空")
	}

	var account *model.AccountBinding
	var err error
	if submission.AccountID > 0 {
		account, err = s.GetBiliAccountByID(userID, submission.AccountID)
	} else {
		account, err = s.GetPrimaryBiliAccountWithFile(userID)
	}
	if err != nil || account == nil {
		s.logger.Error("❌ 没有可用的B站账号，请先绑定B站账号")
		return nil, fmt.Errorf("没有可用的B站账号")
	}

	biliData, _ := account.GetBiliData()
	var biliMid int64
	if biliData != nil {
		biliMid = biliData.BiliMid
	}

	s.logger.Info("✓ 使用B站账号",
		zap.Int64("bili_mid", biliMid),
		zap.String("bili_name", account.Username))

	loginInfo, err := s.loginInfoFromAccount(account)
	if err != nil {
		return nil, err
	}

	uploadClient := bilisdk.NewUploadClient(loginInfo)
	s.logger.Info("⬆️  开始上传视频到B站...", zap.String("path", videoPath))

	coverURL, err := s.resolveSubmissionCover(uploadClient, submission.Cover)
	if err != nil {
		s.logger.Warn("封面上传失败，将忽略自定义封面继续投稿",
			zap.String("cover", submission.Cover),
			zap.Error(err))
		coverURL = ""
	}

	uploadedVideo, err := uploadClient.UploadVideo(videoPath)
	if err != nil {
		return nil, fmt.Errorf("%s", formatUploadError("上传视频", err))
	}

	s.logger.Info("✓ 视频上传成功",
		zap.String("filename", uploadedVideo.Filename),
		zap.String("title", uploadedVideo.Title))

	recommendedTags := s.recommendSubmissionTags(loginInfo.GetCookieString(), submission, uploadedVideo.Filename, coverURL)
	if err := s.persistRecommendedTags(userID, videoPath, recommendedTags); err != nil {
		s.logger.Warn("持久化B站推荐标签失败",
			zap.String("user_id", strings.TrimSpace(userID)),
			zap.String("video_path", strings.TrimSpace(videoPath)),
			zap.Error(err))
	}
	if mergedTags := mergeSubmissionTags(submission.Tag, recommendedTags); mergedTags != "" {
		submission.Tag = mergedTags
	}

	studio := &bilisdk.Studio{
		Copyright: submission.Copyright,
		Source:    submission.Source,
		Title:     submission.Title,
		Desc:      submission.Desc,
		Tag:       submission.Tag,
		Tid:       submission.Tid,
		Cover:     coverURL,
		Videos: []bilisdk.Video{
			*uploadedVideo,
		},
	}

	s.logger.Info("📤 提交稿件到B站...")
	result, err := uploadClient.SubmitVideo(studio)
	if err != nil {
		return nil, fmt.Errorf("%s", formatUploadError("提交稿件", err))
	}
	if result.Code != 0 {
		return nil, fmt.Errorf("提交稿件失败: %s", result.Message)
	}

	bvid, aid := parseSubmissionIDs(result)

	if err := s.UpdateLastUsed(account); err != nil {
		s.logger.Warn("更新账号最后使用时间失败", zap.Error(err))
	}

	return &UploadOutcome{
		BVID:       bvid,
		AID:        aid,
		Filename:   uploadedVideo.Filename,
		VideoTitle: uploadedVideo.Title,
	}, nil
}

func (s *Service) recommendSubmissionTags(cookies string, submission *UploadSubmission, filename, coverURL string) []string {
	if submission == nil {
		return nil
	}

	client := bilisdk.NewClient()
	request := &bilisdk.TagRecommendRequest{
		SubtypeID:   submission.Tid,
		Title:       strings.TrimSpace(submission.Title),
		Filename:    strings.TrimSpace(filename),
		Description: strings.TrimSpace(submission.Desc),
		CoverURL:    strings.TrimSpace(coverURL),
	}

	if request.Title == "" || request.Description == "" {
		return nil
	}
	if request.Filename == "" {
		request.Filename = request.Title
	}

	tags, err := client.RecommendTags(request, strings.TrimSpace(cookies))
	if err != nil {
		s.logger.Warn("获取B站推荐标签失败，继续使用现有标签",
			zap.String("title", request.Title),
			zap.Int("tid", request.SubtypeID),
			zap.Error(err))
		return nil
	}

	result := make([]string, 0, len(tags))
	for _, tag := range tags {
		name := strings.TrimSpace(tag.Name)
		if name == "" {
			name = strings.TrimSpace(tag.Tag)
		}
		if name == "" {
			continue
		}
		result = append(result, name)
	}

	if len(result) > 0 {
		s.logger.Info("✓ 获取B站推荐标签成功", zap.Strings("tags", result))
	}
	return result
}

func mergeSubmissionTags(existing string, recommended []string) string {
	seen := make(map[string]struct{}, 10)
	merged := make([]string, 0, 10)

	appendTag := func(tag string) {
		normalized := strings.TrimSpace(tag)
		if normalized == "" {
			return
		}
		if _, ok := seen[normalized]; ok {
			return
		}
		seen[normalized] = struct{}{}
		merged = append(merged, normalized)
	}

	for _, tag := range strings.Split(existing, ",") {
		appendTag(tag)
	}
	for _, tag := range recommended {
		appendTag(tag)
	}

	return bilisdk.FormatTags(merged)
}

func (s *Service) persistRecommendedTags(userID, videoPath string, tags []string) error {
	formattedTags := bilisdk.FormatTags(tags)
	if err := s.persistRecommendedTagsToDatabase(userID, videoPath, formattedTags); err != nil {
		return err
	}
	if err := s.persistRecommendedTagsToMeta(videoPath, tags); err != nil {
		return err
	}
	return nil
}

func (s *Service) persistRecommendedTagsToDatabase(userID, videoPath, formattedTags string) error {
	if s == nil || s.db == nil {
		return nil
	}
	trimmedPath := strings.TrimSpace(videoPath)
	trimmedUserID := strings.TrimSpace(userID)
	if trimmedPath == "" || trimmedUserID == "" {
		return nil
	}

	result := s.db.Model(&model.Video{}).
		Where("user_id = ? AND video_path = ?", trimmedUserID, trimmedPath).
		Updates(map[string]any{
			"recommended_tags": formattedTags,
			"updated_at":       time.Now(),
		})
	if result.Error != nil {
		return fmt.Errorf("update recommended_tags: %w", result.Error)
	}
	return nil
}

func (s *Service) persistRecommendedTagsToMeta(videoPath string, tags []string) error {
	trimmedPath := strings.TrimSpace(videoPath)
	if trimmedPath == "" {
		return nil
	}

	metaFilePath := filepath.Join(filepath.Dir(trimmedPath), "meta.json")
	payload := map[string]any{}
	if data, err := os.ReadFile(metaFilePath); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &payload); err != nil {
			return fmt.Errorf("unmarshal meta.json: %w", err)
		}
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read meta.json: %w", err)
	}

	payload["recommended_tags"] = normalizeRecommendedTags(tags)
	payload["recommended_tags_updated_at"] = time.Now().Format("2006-01-02 15:04:05")

	jsonData, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal meta.json: %w", err)
	}
	if err := os.WriteFile(metaFilePath, jsonData, 0644); err != nil {
		return fmt.Errorf("write meta.json: %w", err)
	}
	return nil
}

func normalizeRecommendedTags(tags []string) []string {
	result := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		trimmed := strings.TrimSpace(tag)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func (s *Service) resolveSubmissionCover(uploader coverUploader, cover string) (string, error) {
	cover = strings.TrimSpace(cover)
	if cover == "" {
		return "", nil
	}
	if strings.HasPrefix(cover, "http://") || strings.HasPrefix(cover, "https://") {
		uploadedCoverURL, err := uploader.UploadCoverFromURL(cover)
		if err != nil {
			return "", fmt.Errorf("upload remote cover: %w", err)
		}
		return strings.TrimSpace(uploadedCoverURL), nil
	}

	info, err := os.Stat(cover)
	if err != nil {
		return "", fmt.Errorf("stat local cover: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("local cover path is a directory: %s", cover)
	}

	uploadedCoverURL, err := uploader.UploadCover(cover)
	if err != nil {
		return "", fmt.Errorf("upload local cover: %w", err)
	}
	return strings.TrimSpace(uploadedCoverURL), nil
}

func (s *Service) loginInfoFromAccount(account *model.AccountBinding) (*bilisdk.LoginInfo, error) {
	cookiesJSON, err := s.GetDecryptedCookies(account)
	if err != nil {
		return nil, fmt.Errorf("解密登录凭证失败: %w", err)
	}

	var loginInfo bilisdk.LoginInfo
	if err := json.Unmarshal([]byte(cookiesJSON), &loginInfo); err != nil {
		return nil, fmt.Errorf("解析登录信息失败（请重新绑定B站账号）: %w", err)
	}

	return &loginInfo, nil
}

func parseSubmissionIDs(result *bilisdk.ResponseData) (string, int64) {
	if result == nil || result.Data == nil {
		return "", 0
	}

	dataMap, ok := result.Data.(map[string]interface{})
	if !ok {
		return "", 0
	}

	var bvid string
	var aid int64
	if bvidVal, exists := dataMap["bvid"]; exists {
		if bvidStr, ok := bvidVal.(string); ok {
			bvid = bvidStr
		}
	}
	if aidVal, exists := dataMap["aid"]; exists {
		switch value := aidVal.(type) {
		case float64:
			aid = int64(value)
		case int64:
			aid = value
		case int:
			aid = int64(value)
		}
	}

	return bvid, aid
}

func formatUploadError(operation string, err error) string {
	errorStr := err.Error()

	if strings.Contains(errorStr, "broken pipe") || strings.Contains(errorStr, "connection reset") {
		return fmt.Sprintf("%s失败：网络连接中断，请检查网络状态后重试", operation)
	}
	if strings.Contains(errorStr, "timeout") || strings.Contains(errorStr, "deadline exceeded") {
		return fmt.Sprintf("%s失败：网络超时，请稍后重试", operation)
	}
	if strings.Contains(errorStr, "connection refused") {
		return fmt.Sprintf("%s失败：无法连接到B站服务器，请检查网络连接", operation)
	}
	if strings.Contains(errorStr, "no such host") || strings.Contains(errorStr, "dns") {
		return fmt.Sprintf("%s失败：网络域名解析失败，请检查网络设置", operation)
	}

	if strings.Contains(errorStr, "no such file") || strings.Contains(errorStr, "file not found") {
		return fmt.Sprintf("%s失败：找不到视频文件，请确认文件已正确下载", operation)
	}
	if strings.Contains(errorStr, "permission denied") {
		return fmt.Sprintf("%s失败：文件访问权限不足", operation)
	}
	if strings.Contains(errorStr, "file too large") {
		return fmt.Sprintf("%s失败：文件过大，超出B站上传限制", operation)
	}

	if strings.Contains(errorStr, "401") || strings.Contains(errorStr, "unauthorized") {
		return fmt.Sprintf("%s失败：登录状态已过期，请重新登录", operation)
	}
	if strings.Contains(errorStr, "403") || strings.Contains(errorStr, "forbidden") {
		return fmt.Sprintf("%s失败：账号权限不足或被限制", operation)
	}
	if strings.Contains(errorStr, "429") || strings.Contains(errorStr, "rate limit") {
		return fmt.Sprintf("%s失败：操作频率过快，请稍后再试", operation)
	}
	if strings.Contains(errorStr, "500") || strings.Contains(errorStr, "internal server error") {
		return fmt.Sprintf("%s失败：B站服务器临时异常，请稍后重试", operation)
	}

	return fmt.Sprintf("%s失败：%v", operation, err)
}

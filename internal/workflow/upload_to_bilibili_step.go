package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/zolagz/ytb2bili/internal/service"
	biliaccount "github.com/zolagz/ytb2bili/pkg/bilibili"
	"github.com/zolagz/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const defaultBilibiliSubmissionTID = 122
const preferredBilibiliCoverFilename = "thumbnail_maxresdefault.jpg"

// ============================================================================
// 上传到B站步骤
// ============================================================================

// UploadToBilibiliStep 上传视频到B站的步骤
type UploadToBilibiliStep struct {
	BaseStep
	accountService *biliaccount.Service
	userSettings   *service.UserSettingsClient
	db             *gorm.DB
	logger         *zap.Logger
}

// NewUploadToBilibiliStep 创建上传到B站的步骤
func NewUploadToBilibiliStep(
	accountService *biliaccount.Service,
	userSettings *service.UserSettingsClient,
	db *gorm.DB,
	logger *zap.Logger,
) *UploadToBilibiliStep {
	return &UploadToBilibiliStep{
		BaseStep:       NewBaseStepWithOrder(StepNameUploadToBilibili, true, 100),
		accountService: accountService,
		userSettings:   userSettings,
		db:             db,
		logger:         logger,
	}
}

// Execute 执行上传到B站
func (s *UploadToBilibiliStep) Execute(ctx context.Context, input any) (any, error) {
	vctx, err := mustVideoContext(input)
	if err != nil {
		return nil, err
	}

	s.logger.Info("========================================")
	s.logger.Info("开始上传视频到 Bilibili")
	s.logger.Info("========================================")

	// 1. 获取当前用户ID（优先从 workflow context 读取，兼容旧的 "user_id" key）
	userID := strings.TrimSpace(GetUserID(ctx))
	if userID == "" {
		if legacy, ok := ctx.Value("user_id").(string); ok {
			userID = strings.TrimSpace(legacy)
		}
	}
	if userID == "" {
		return nil, fmt.Errorf("未找到用户ID，请先登录")
	}
	vctx.UserID = userID

	// 2. 查找视频文件
	videoFiles := s.findVideoFiles(vctx.VideoPath)
	if len(videoFiles) == 0 {
		return nil, fmt.Errorf("未找到视频文件")
	}

	videoFile := videoFiles[0]
	s.logger.Info("📹 找到视频文件", zap.String("path", videoFile))

	// 3. 构建投稿信息并上传
	submission := s.buildSubmissionInfo(ctx, userID, vctx)
	result, err := s.accountService.UploadVideoForUser(userID, videoFile, submission)
	if err != nil {
		s.logger.Error("❌ B站上传失败", zap.Error(err))
		return nil, err
	}

	vctx.BiliBVID = result.BVID
	vctx.BiliAID = result.AID
	if result.BVID != "" {
		s.logger.Info("📺 BVID: " + result.BVID)
	}
	if result.AID > 0 {
		s.logger.Info("📺 AID: ", zap.Int64("aid", result.AID))
	}

	s.logger.Info("========================================")
	s.logger.Info("✅ 视频已成功上传到B站!")
	if result.BVID != "" {
		s.logger.Info("🔗 视频链接: https://www.bilibili.com/video/" + result.BVID)
	}
	s.logger.Info("========================================")

	return vctx, nil
}

// OnSuccess 成功后的回调
func (s *UploadToBilibiliStep) OnSuccess(ctx context.Context, output any) error {
	vctx := output.(*VideoContext)
	s.logger.Info("✓ Upload to Bilibili completed",
		zap.String("bvid", vctx.BiliBVID),
		zap.Int64("aid", vctx.BiliAID))
	return nil
}

// findVideoFiles 查找视频文件
func (s *UploadToBilibiliStep) findVideoFiles(videoPath string) []string {
	var videoFiles []string
	videoExtensions := []string{".mp4", ".flv", ".mkv", ".webm", ".avi", ".mov"}

	// 如果videoPath本身就是视频文件
	ext := strings.ToLower(filepath.Ext(videoPath))
	for _, videoExt := range videoExtensions {
		if ext == videoExt {
			return []string{videoPath}
		}
	}

	// 否则在videoPath所在目录查找
	dir := filepath.Dir(videoPath)
	files, err := os.ReadDir(dir)
	if err != nil {
		s.logger.Error("读取目录失败", zap.Error(err))
		return videoFiles
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(file.Name()))
		for _, videoExt := range videoExtensions {
			if ext == videoExt {
				fullPath := filepath.Join(dir, file.Name())
				videoFiles = append(videoFiles, fullPath)
				break
			}
		}
	}

	return videoFiles
}

// buildSubmissionInfo 构建投稿信息
func (s *UploadToBilibiliStep) buildSubmissionInfo(ctx context.Context, userID string, vctx *VideoContext) *biliaccount.UploadSubmission {
	// 默认值
	title := vctx.VideoID
	desc := "自动上传的视频"
	tags := "视频"
	coverURL := ""

	// 优先使用生成的元数据（来自 GenerateMetadataStep）
	if vctx.Title != "" {
		title = vctx.Title
		s.logger.Info("✓ 使用AI生成的标题", zap.String("title", title))
	}

	if vctx.Description != "" {
		desc = vctx.Description
		s.logger.Info("✓ 使用AI生成的描述")
	}

	if vctx.Tags != "" {
		tags = vctx.Tags
		s.logger.Info("✓ 使用AI生成的标签", zap.String("tags", tags))
	}

	// 如果没有生成的元数据，从数据库查询
	if title == vctx.VideoID {
		var savedVideo model.Video
		err := s.db.Where("video_id = ?", vctx.VideoID).First(&savedVideo).Error
		if err == nil {
			// 清理标题中的标签
			if savedVideo.Title != "" {
				title = s.cleanTitle(savedVideo.Title)
			}

			// 构建描述
			if savedVideo.Description != "" {
				desc = savedVideo.Description
			}
		}
	}

	coverURL = s.resolveSubmissionCoverPath(vctx)

	// 从用户设置获取投稿分区，未设置时保持现有默认值。
	tid := s.resolveSubmissionTID(ctx, userID)
	copyright := s.resolveSubmissionCopyright(ctx, userID)

	var source string
	if copyright == model.DefaultBilibiliSubmissionCopyright {
		if vctx.VideoURL != "" {
			desc += fmt.Sprintf("\n\n原视频链接：%s\n🔄 本视频为转载内容，仅供学习交流使用", vctx.VideoURL)
		} else {
			desc += "\n\n🔄 本视频为转载内容，仅供学习交流使用"
		}
		source = strings.TrimSpace(vctx.VideoURL)
		if source == "" {
			source = "https://www.youtube.com"
		}
	// } else if vctx.VideoURL != "" {
	// 	desc += fmt.Sprintf("\n\n原视频链接：%s", vctx.VideoURL)
	}

	const maxDescLength = 2000
	if len([]rune(desc)) > maxDescLength {
		desc = string([]rune(desc)[:maxDescLength-3]) + "..."
	}

	studio := &biliaccount.UploadSubmission{
		AccountID:  vctx.BiliAccountID,
		Copyright: copyright,
		Source:    source,
		Title:     s.truncateTitle(title, 80), // B站标题最长80字符
		Desc:      desc,
		Tag:       tags,
		Tid:       tid,
		Cover:     coverURL,
	}

	s.logger.Info("📋 投稿信息:",
		zap.String("标题", studio.Title),
		zap.String("简介", s.truncateString(studio.Desc, 100)),
		zap.String("标签", studio.Tag),
		zap.Int("分区", studio.Tid),
		zap.Int("稿件类型", studio.Copyright))

	return studio
}

func (s *UploadToBilibiliStep) resolveSubmissionCoverPath(vctx *VideoContext) string {
	if vctx == nil {
		return ""
	}

	if videoPath := strings.TrimSpace(vctx.VideoPath); videoPath != "" {
		preferredCoverPath := filepath.Join(filepath.Dir(videoPath), preferredBilibiliCoverFilename)
		if info, err := os.Stat(preferredCoverPath); err == nil && !info.IsDir() {
			s.logger.Info("✓ 使用视频同级目录封面", zap.String("path", preferredCoverPath))
			return preferredCoverPath
		}
	}

	thumbnailPath := strings.TrimSpace(vctx.ThumbnailPath)
	if thumbnailPath != "" {
		s.logger.Info("✓ 使用工作流封面", zap.String("path", thumbnailPath))
	}
	return thumbnailPath
}

func (s *UploadToBilibiliStep) resolveSubmissionTID(ctx context.Context, userID string) int {
	if s.userSettings == nil || strings.TrimSpace(userID) == "" {
		return defaultBilibiliSubmissionTID
	}

	settings, err := s.userSettings.GetSettings(ctx, userID)
	if err != nil {
		s.logger.Warn("读取用户投稿分区设置失败，使用默认分区",
			zap.String("user_id", userID),
			zap.Error(err))
		return defaultBilibiliSubmissionTID
	}

	rawTID := strings.TrimSpace(settings[model.UserSettingKeyBilibiliSubmissionTID])
	if rawTID == "" {
		return defaultBilibiliSubmissionTID
	}

	tid, err := strconv.Atoi(rawTID)
	if err != nil || tid <= 0 {
		s.logger.Warn("用户投稿分区设置无效，使用默认分区",
			zap.String("user_id", userID),
			zap.String("tid", rawTID))
		return defaultBilibiliSubmissionTID
	}

	return tid
}

func (s *UploadToBilibiliStep) resolveSubmissionCopyright(ctx context.Context, userID string) int {
	if override, ok := ctx.Value(bilibiliSubmissionCopyrightContextKey{}).(int); ok && model.IsAllowedBilibiliSubmissionCopyright(override) {
		return override
	}

	if s.userSettings == nil || strings.TrimSpace(userID) == "" {
		return model.DefaultBilibiliSubmissionCopyright
	}

	settings, err := s.userSettings.GetSettings(ctx, userID)
	if err != nil {
		s.logger.Warn("读取用户投稿类型设置失败，使用默认投稿类型",
			zap.String("user_id", userID),
			zap.Error(err))
		return model.DefaultBilibiliSubmissionCopyright
	}

	rawCopyright := strings.TrimSpace(settings[model.UserSettingKeyBilibiliSubmissionCopyright])
	if rawCopyright == "" {
		return model.DefaultBilibiliSubmissionCopyright
	}

	copyright, err := strconv.Atoi(rawCopyright)
	if err != nil || !model.IsAllowedBilibiliSubmissionCopyright(copyright) {
		s.logger.Warn("用户投稿类型设置无效，使用默认投稿类型",
			zap.String("user_id", userID),
			zap.String("copyright", rawCopyright))
		return model.DefaultBilibiliSubmissionCopyright
	}

	return copyright
}

// cleanTitle 清理标题中的标签
func (s *UploadToBilibiliStep) cleanTitle(title string) string {
	// 移除 #hashtag
	re := regexp.MustCompile(`\s*#[^\s#]+`)
	cleaned := re.ReplaceAllString(title, "")

	// 清理多余的空格
	cleaned = strings.TrimSpace(cleaned)
	re2 := regexp.MustCompile(`\s+`)
	cleaned = re2.ReplaceAllString(cleaned, " ")

	return cleaned
}

// truncateTitle 截断标题
func (s *UploadToBilibiliStep) truncateTitle(title string, maxLen int) string {
	runes := []rune(title)
	if len(runes) <= maxLen {
		return title
	}
	return string(runes[:maxLen-3]) + "..."
}

// truncateString 截断字符串
func (s *UploadToBilibiliStep) truncateString(str string, maxLen int) string {
	runes := []rune(str)
	if len(runes) <= maxLen {
		return str
	}
	return string(runes[:maxLen-3]) + "..."
}

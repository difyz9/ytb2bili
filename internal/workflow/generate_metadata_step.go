package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zolagz/ytb2bili/internal/service"
	biliaccount "github.com/zolagz/ytb2bili/pkg/bilibili"
	"github.com/zolagz/ytb2bili/pkg/llm"
	storemodel "github.com/zolagz/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ============================================================================
// 视频元数据生成步骤
// ============================================================================

// GenerateMetadataStep 生成视频标题、描述和标签
type GenerateMetadataStep struct {
	llmClient          *llm.EinoChatClient
	userSettings       *service.UserSettingsClient
	biliAccountService *biliaccount.Service
	db                 *gorm.DB
	logger             *zap.Logger
}

// VideoMetadata 视频元数据
type VideoMetadata struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

const watermarkPromoCopy = "ytb2bili\n全链路自动化视频工具\n下载、转写、翻译、配音、音画精准同步"

// NewGenerateMetadataStep 创建元数据生成步骤
func NewGenerateMetadataStep(
	llmClient *llm.EinoChatClient,
	userSettings *service.UserSettingsClient,
	accountService *biliaccount.Service,
	db *gorm.DB,
	logger *zap.Logger,
) *GenerateMetadataStep {
	return &GenerateMetadataStep{
		llmClient:          llmClient,
		userSettings:       userSettings,
		biliAccountService: accountService,
		db:                 db,
		logger:             logger,
	}
}

func (s *GenerateMetadataStep) resolveLLMClient(ctx context.Context, userID, modelName string) (*llm.EinoChatClient, error) {
	if s.llmClient == nil {
		return nil, fmt.Errorf("LLM client not configured")
	}
	return s.llmClient, nil
}

func (s *GenerateMetadataStep) resolveDatabaseLLMClient(ctx context.Context, modelName string) (*llm.EinoChatClient, error) {
	if s.llmClient == nil {
		return nil, fmt.Errorf("LLM client not configured")
	}
	return s.llmClient, nil
}

func isUnauthorizedLLMError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	if strings.Contains(msg, "401") {
		return true
	}
	if strings.Contains(msg, "Unauthorized") {
		return true
	}
	if strings.Contains(msg, "无效的 API Key") {
		return true
	}
	return false
}

func (s *GenerateMetadataStep) resolveMetadataModel(ctx context.Context, userID string) string {
	if s.llmClient != nil {
		return s.llmClient.ModelName()
	}
	return llm.DefaultModel
}

func (s *GenerateMetadataStep) resolveWatermarkPromoEnabled(ctx context.Context, userID string) bool {
	if s.userSettings == nil || !s.userSettings.IsEnabled() || strings.TrimSpace(userID) == "" {
		return true
	}
	settings, err := s.userSettings.GetSettings(ctx, userID)
	if err != nil {
		s.logger.Warn("加载上传宣传文案设置失败，回退默认开启", zap.String("user_id", userID), zap.Error(err))
		return true
	}
	return strings.TrimSpace(settings[storemodel.UserSettingKeyWatermarkPromoEnabled]) != "0"
}

// Execute 执行元数据生成
func (s *GenerateMetadataStep) Execute(ctx context.Context, input any) (any, error) {
	vctx, ok := input.(*VideoContext)
	if !ok {
		return nil, fmt.Errorf("无效的输入类型: 期望 *VideoContext")
	}

	// 优先从 VideoContext / DB 取到原始标题，供 LLM 生成“中文标题”参考。
	originalTitle := s.resolveOriginalVideoTitle(ctx, vctx)
	promoEnabled := s.resolveWatermarkPromoEnabled(ctx, vctx.UserID)
	if strings.TrimSpace(vctx.Title) == "" && strings.TrimSpace(originalTitle) != "" {
		vctx.Title = strings.TrimSpace(originalTitle)
	}

	persistFallback := func(reason string) (*VideoContext, error) {
		if strings.TrimSpace(reason) != "" {
			s.logger.Warn("⚠️  使用默认元数据", zap.String("reason", reason))
		}
		vctx, err := s.useDefaultMetadata(vctx, promoEnabled)
		if err != nil {
			return vctx, err
		}
		fallback := &VideoMetadata{
			Title:       strings.TrimSpace(vctx.Title),
			Description: ensurePromoCopy(vctx.Description, promoEnabled),
			Tags:        splitAndTrimCommaSeparated(vctx.Tags),
		}
		fallback.Description = ensurePromoCopy(fallback.Description, promoEnabled)
		// 默认/兜底分支也写入 DB，确保前端“编辑上传”能读到 generated_*。
		if s.db != nil {
			if dbErr := s.saveMetadataToDatabase(vctx, fallback); dbErr != nil {
				s.logger.Warn("⚠️  保存默认元数据到数据库失败", zap.Error(dbErr))
			}
		}
		if strings.TrimSpace(vctx.VideoPath) != "" {
			if fileErr := s.saveMetadataToFile(vctx.VideoPath, fallback); fileErr != nil {
				s.logger.Warn("⚠️  保存默认 meta.json 失败", zap.Error(fileErr))
			}
		}
		// 回写到上下文（用于后续上传步骤）
		vctx.Title = fallback.Title
		vctx.Description = fallback.Description
		vctx.Tags = strings.Join(fallback.Tags, ",")
		return vctx, nil
	}

	s.logger.Info("========================================")
	s.logger.Info("开始生成视频元数据")
	s.logger.Info("========================================")

	// 1. 查找字幕文件
	subtitlePath := s.findSubtitleFile(vctx)
	if subtitlePath == "" {
		return persistFallback("未找到字幕文件")
	}

	s.logger.Info("📝 找到字幕文件", zap.String("path", subtitlePath))

	// 2. 读取字幕内容
	subtitleContent, err := os.ReadFile(subtitlePath)
	if err != nil {
		s.logger.Error("❌ 读取字幕文件失败", zap.Error(err))
		return persistFallback("读取字幕文件失败")
	}

	// 3. 提取字幕文本
	subtitleText := s.extractTextFromSubtitle(string(subtitleContent))
	if subtitleText == "" {
		return persistFallback("字幕内容为空")
	}

	// 截取字幕文本（避免token过多）
	maxLength := 2000
	if len(subtitleText) > maxLength {
		subtitleText = subtitleText[:maxLength] + "..."
	}

	s.logger.Info("📄 提取字幕文本完成", zap.Int("length", len(subtitleText)))

	// 4. 调用LLM生成元数据
	s.logger.Info("🤖 调用LLM生成视频标题、描述和标签...")
	resolvedModel := s.resolveMetadataModel(ctx, vctx.UserID)
	s.logger.Info("resolved metadata model",
		zap.String("model", resolvedModel),
		zap.String("video_path", vctx.VideoPath))
	metadata, err := s.generateMetadataFromLLM(ctx, vctx.UserID, resolvedModel, originalTitle, subtitleText, promoEnabled)
	if err != nil {
		s.logger.Error("❌ LLM生成元数据失败", zap.Error(err))
		return persistFallback("LLM生成失败")
	}

	metadata.Description = ensurePromoCopy(metadata.Description, promoEnabled)

	// 5. 验证和调整标题长度（B站限制80字符）
	if len([]rune(metadata.Title)) > 80 {
		runes := []rune(metadata.Title)
		metadata.Title = string(runes[:77]) + "..."
		s.logger.Warn("⚠️  标题过长，已截断为80字符")
	}

	// 6. 保存到上下文
	vctx.Title = metadata.Title
	vctx.Description = metadata.Description
	vctx.Tags = strings.Join(metadata.Tags, ",")

	// 7. 保存到数据库
	if s.db != nil {
		if err := s.saveMetadataToDatabase(vctx, metadata); err != nil {
			s.logger.Error("❌ 保存元数据到数据库失败", zap.Error(err))
			// 不影响流程继续
		}
	}

	// 8. 保存到meta.json文件（可选）
	if strings.TrimSpace(vctx.VideoPath) != "" {
		if err := s.saveMetadataToFile(vctx.VideoPath, metadata); err != nil {
			s.logger.Warn("⚠️  保存meta.json失败", zap.Error(err))
		}
	}

	s.logger.Info("========================================")
	s.logger.Info("✅ 视频元数据生成成功")
	s.logger.Info("📌 标题:", zap.String("title", metadata.Title))
	s.logger.Info("📝 描述:", zap.String("desc", s.truncateString(metadata.Description, 100)))
	s.logger.Info("🏷️  标签:", zap.String("tags", strings.Join(metadata.Tags, ", ")))
	s.logger.Info("========================================")

	return vctx, nil
}

// findSubtitleFile 查找字幕文件
func (s *GenerateMetadataStep) findSubtitleFile(vctx *VideoContext) string {
	videoPath := strings.TrimSpace(vctx.VideoPath)
	if videoPath == "" {
		return ""
	}

	videoDir := filepath.Dir(videoPath)
	videoBaseName := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	videoID := strings.TrimSpace(vctx.VideoID)

	// 翻译后的中文字幕优先（LLMTranslateStep 会生成 videoID.zh.srt / videoID.srt，且内容为中文）
	preferred := make([]string, 0, 16)
	preferred = append(preferred, filepath.Join(videoDir, "zh.srt"))
	if videoID != "" {
		preferred = append(preferred,
			filepath.Join(videoDir, videoID+".zh.srt"),
			filepath.Join(videoDir, videoID+".zh-CN.srt"),
			filepath.Join(videoDir, videoID+".zh-Hans.srt"),
			filepath.Join(videoDir, videoID+".srt"),
		)
	}
	preferred = append(preferred,
		filepath.Join(videoDir, videoBaseName+".zh.srt"),
		filepath.Join(videoDir, videoBaseName+".zh-CN.srt"),
		filepath.Join(videoDir, videoBaseName+".zh-Hans.srt"),
		filepath.Join(videoDir, videoBaseName+".srt"),
	)

	for _, candidate := range preferred {
		if candidate == "" {
			continue
		}
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() && st.Size() > 0 {
			return candidate
		}
	}

	// 兜底：从目录里挑一个“最像中文译稿”的 srt（避免误选 *.en.srt）
	matches, _ := filepath.Glob(filepath.Join(videoDir, "*.srt"))
	bestPath := ""
	bestScore := -1 << 30
	for _, match := range matches {
		st, err := os.Stat(match)
		if err != nil || st.IsDir() || st.Size() == 0 {
			continue
		}
		score := scoreSubtitlePath(match, videoID, videoBaseName)
		if score > bestScore {
			bestScore = score
			bestPath = match
		}
	}

	return bestPath
}

func scoreSubtitlePath(path, videoID, videoBaseName string) int {
	name := strings.ToLower(filepath.Base(path))
	score := 0

	if name == "zh.srt" {
		score += 10_000
	}
	if strings.HasSuffix(name, ".zh.srt") || strings.Contains(name, ".zh-") || strings.Contains(name, ".zh_") {
		score += 9_000
	}
	if strings.HasSuffix(name, ".zh-cn.srt") || strings.HasSuffix(name, ".zh-hans.srt") {
		score += 9_500
	}

	// 翻译 step 会写 videoID.srt（中文），也给较高权重
	if videoID != "" {
		vid := strings.ToLower(videoID)
		if strings.Contains(name, vid+".zh.srt") {
			score += 9_800
		}
		if strings.HasSuffix(name, vid+".srt") {
			score += 8_000
		}
		if strings.Contains(name, vid) {
			score += 200
		}
	}
	if videoBaseName != "" {
		bn := strings.ToLower(videoBaseName)
		if strings.Contains(name, bn) {
			score += 100
		}
	}

	// 强力排除英文/原文字幕
	if strings.Contains(name, ".en.") || strings.HasSuffix(name, ".en.srt") || name == "en.srt" || strings.Contains(name, "en-us") || strings.Contains(name, "en_us") {
		score -= 20_000
	}

	// 其他 srt 给基础分
	if strings.HasSuffix(name, ".srt") {
		score += 50
	}

	return score
}

// extractTextFromSubtitle 从字幕内容中提取纯文本
func (s *GenerateMetadataStep) extractTextFromSubtitle(content string) string {
	lines := strings.Split(content, "\n")
	var textLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// 跳过空行、序号行、时间码行
		if line == "" || s.isNumber(line) || strings.Contains(line, "-->") {
			continue
		}
		textLines = append(textLines, line)
	}

	return strings.Join(textLines, " ")
}

// isNumber 检查字符串是否为纯数字
func (s *GenerateMetadataStep) isNumber(str string) bool {
	for _, c := range str {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(str) > 0
}

// generateMetadataFromLLM 使用LLM生成元数据
func (s *GenerateMetadataStep) generateMetadataFromLLM(ctx context.Context, userID, modelName, originalTitle, subtitleText string, promoEnabled bool) (*VideoMetadata, error) {
	originalTitle = strings.TrimSpace(originalTitle)
	titleHint := ""
	if originalTitle != "" {
		titleHint = fmt.Sprintf("\n原始视频标题（可能非中文）：\n%s\n", originalTitle)
	}
	promoRequirement := ""
	if promoEnabled {
		promoRequirement = fmt.Sprintf("5. 视频介绍中必须包含以下宣传语（原样保留换行）：\n%s\n\n", watermarkPromoCopy)
	}

	prompt := fmt.Sprintf(`请根据以下“翻译后的中文字幕（简体中文）”内容，生成一个吸引人的B站视频标题、符合B站投稿习惯的视频简介、以及3-5个相关标签。%s

字幕内容：
%s

要求：
1. 标题要简洁有力，严格控制在30个字以内，能够准确概括视频主题，吸引观众点击；如果提供了“原始视频标题”，请在忠实表达原意的基础上翻译/改写为自然的中文标题
2. 简介要符合B站投稿习惯：结构清晰、可读性强，可以包含小标题与换行；不要站外导流（不要出现微信/QQ/群/链接等）；避免夸大、低俗、违法违规、敏感词
3. 简介建议包含：一句话概述 + 3-5条要点（用换行分隔）+ 适合人群/看点（1-2行）
3. 标签要准确反映视频内容，3-5个即可
4. 必须使用中文
6. 输出格式必须是JSON，格式如下：
{
  "title": "视频标题",
	"description": "视频介绍",
  "tags": ["标签1", "标签2", "标签3"]
}

%s请直接返回JSON格式的结果，不要包含任何其他说明文字。`, titleHint, subtitleText, promoRequirement)
	prompt = strings.ReplaceAll(prompt, "\t", "")

	// 构建消息
	messages := []llm.Message{
		{
			Role:    "system",
			Content: "你是一个专业的B站UP主助手，擅长根据视频内容生成吸引人的标题和描述。",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}

	// 调用LLM
	llmClient, err := s.resolveLLMClient(ctx, userID, modelName)
	if err != nil {
		return nil, err
	}

	response, err := llmClient.ChatWithOptions(ctx, messages, llm.ChatOptions{
		Model: modelName,
	})
	if err != nil && isUnauthorizedLLMError(err) {
		// 如果请求上下文 token 失效，参考字幕翻译的认证方式：回退到数据库中最近的有效 API key 再试一次。
		fallback, fbErr := s.resolveDatabaseLLMClient(ctx, modelName)
		if fbErr == nil {
			llmClient = fallback
			response, err = llmClient.ChatWithOptions(ctx, messages, llm.ChatOptions{Model: modelName})
		}
	}
	if err != nil {
		return nil, fmt.Errorf("LLM调用失败: %w", err)
	}

	s.logger.Debug("LLM原始响应", zap.String("response", response))

	// 解析JSON响应
	metadata, err := s.parseMetadataJSON(response)
	if err != nil {
		return nil, fmt.Errorf("解析LLM响应失败: %w", err)
	}

	return metadata, nil
}

// parseMetadataJSON 解析JSON格式的元数据
func (s *GenerateMetadataStep) parseMetadataJSON(content string) (*VideoMetadata, error) {
	var metadata VideoMetadata

	// 清理可能的markdown代码块标记
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSuffix(content, "```")
	} else if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
	}
	content = strings.TrimSpace(content)

	// 解析JSON
	if err := json.Unmarshal([]byte(content), &metadata); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %w, 内容: %s", err, content)
	}

	// 验证必填字段
	if metadata.Title == "" {
		return nil, fmt.Errorf("生成的标题为空")
	}

	return &metadata, nil
}

// saveMetadataToDatabase 保存元数据到数据库
func (s *GenerateMetadataStep) saveMetadataToDatabase(vctx *VideoContext, metadata *VideoMetadata) error {
	updates := map[string]interface{}{
		"generated_title": metadata.Title,
		"generated_desc":  metadata.Description,
		"generated_tags":  strings.Join(metadata.Tags, ","),
		"updated_at":      time.Now(),
	}

	query := s.db.Model(&storemodel.Video{})
	if strings.TrimSpace(vctx.VideoID) != "" {
		query = query.Where("video_id = ?", vctx.VideoID)
	} else {
		query = query.Where("video_path = ? OR url = ?", vctx.VideoPath, vctx.VideoURL)
	}

	result := query.Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("数据库更新失败: %w", result.Error)
	}

	if result.RowsAffected == 0 && (strings.TrimSpace(vctx.VideoPath) != "" || strings.TrimSpace(vctx.VideoURL) != "") {
		result = s.db.Model(&storemodel.Video{}).
			Where("video_path = ? OR url = ?", vctx.VideoPath, vctx.VideoURL).
			Updates(updates)
		if result.Error != nil {
			return fmt.Errorf("数据库更新失败: %w", result.Error)
		}
	}

	if result.RowsAffected == 0 {
		s.logger.Warn("⚠️  未找到匹配的视频记录，跳过数据库更新",
			zap.String("video_id", strings.TrimSpace(vctx.VideoID)),
			zap.String("video_path", strings.TrimSpace(vctx.VideoPath)),
			zap.String("video_url", strings.TrimSpace(vctx.VideoURL)))
	} else {
		s.logger.Info("✅ 元数据已保存到数据库", zap.Int64("rows_affected", result.RowsAffected))
	}

	return nil
}

// saveMetadataToFile 保存元数据到meta.json文件
func (s *GenerateMetadataStep) saveMetadataToFile(videoPath string, metadata *VideoMetadata) error {
	videoDir := filepath.Dir(videoPath)
	metaFilePath := filepath.Join(videoDir, "meta.json")

	fileMetadata := map[string]interface{}{
		"title":        metadata.Title,
		"description":  metadata.Description,
		"tags":         metadata.Tags,
		"generated_at": time.Now().Format("2006-01-02 15:04:05"),
	}

	jsonData, err := json.MarshalIndent(fileMetadata, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON序列化失败: %w", err)
	}

	if err := os.WriteFile(metaFilePath, jsonData, 0644); err != nil {
		return fmt.Errorf("写入meta.json失败: %w", err)
	}

	s.logger.Info("📁 meta.json已保存", zap.String("path", metaFilePath))
	return nil
}

// useDefaultMetadata 使用默认元数据
func (s *GenerateMetadataStep) useDefaultMetadata(vctx *VideoContext, promoEnabled bool) (*VideoContext, error) {
	// 优先使用上下文已有标题（本地上传/下载步骤可能已填充）
	defaultTitle := strings.TrimSpace(vctx.Title)
	if defaultTitle == "" {
		// 不使用 URL 作为标题兜底，避免把链接当标题展示。
		defaultTitle = "视频"
	}

	defaultDesc := strings.TrimSpace(vctx.Description)
	if defaultDesc == "" {
		defaultDesc = "通过自动化工具上传的视频"
	}

	vctx.Title = defaultTitle
	vctx.Description = ensurePromoCopy(defaultDesc, promoEnabled)
	if strings.TrimSpace(vctx.Tags) == "" {
		vctx.Tags = "视频,自动上传"
	}

	s.logger.Info("✓ 使用默认元数据", zap.String("title", defaultTitle))
	return vctx, nil
}

func (s *GenerateMetadataStep) resolveOriginalVideoTitle(ctx context.Context, vctx *VideoContext) string {
	if vctx == nil {
		return ""
	}
	if title := strings.TrimSpace(vctx.Title); title != "" {
		return title
	}
	if s.db == nil {
		return ""
	}

	query := s.db.WithContext(ctx).Model(&storemodel.Video{})
	if vid := strings.TrimSpace(vctx.VideoID); vid != "" {
		query = query.Where("video_id = ?", vid)
	} else {
		vp := strings.TrimSpace(vctx.VideoPath)
		vu := strings.TrimSpace(vctx.VideoURL)
		if vp == "" && vu == "" {
			return ""
		}
		query = query.Where("video_path = ? OR url = ?", vp, vu)
	}

	var video storemodel.Video
	if err := query.First(&video).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ""
		}
		s.logger.Warn("查询视频标题失败", zap.Error(err))
		return ""
	}
	return strings.TrimSpace(video.Title)
}

func splitAndTrimCommaSeparated(tags string) []string {
	trimmed := strings.TrimSpace(tags)
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

// truncateString 截断字符串用于日志显示
func (s *GenerateMetadataStep) truncateString(str string, maxLen int) string {
	runes := []rune(str)
	if len(runes) <= maxLen {
		return str
	}
	return string(runes[:maxLen]) + "..."
}

func ensurePromoCopy(description string, enabled bool) string {
	if !enabled {
		return strings.TrimSpace(description)
	}
	trimmed := strings.TrimSpace(description)
	if trimmed == "" {
		return watermarkPromoCopy
	}
	// 避免重复追加
	if strings.Contains(trimmed, watermarkPromoCopy) {
		return trimmed
	}
	return trimmed + "\n\n" + watermarkPromoCopy
}

// Name 返回步骤名称
func (s *GenerateMetadataStep) Name() string {
	return StepNameGenerateMetadata
}

// IsRequired 是否为必需步骤
func (s *GenerateMetadataStep) IsRequired() bool {
	return false // 元数据生成失败不影响视频上传
}

// Order 返回执行顺序
func (s *GenerateMetadataStep) Order() int {
	// 在主要处理步骤完成后执行（生成标题/简介/标签），失败不影响主流程。
	return 30
}


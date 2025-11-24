package handlers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/difyz9/ytb2bili/internal/chain_task/base"
	"github.com/difyz9/ytb2bili/internal/chain_task/manager"
	"github.com/difyz9/ytb2bili/internal/core"
	"github.com/difyz9/ytb2bili/internal/core/services"
	"github.com/difyz9/ytb2bili/pkg/cos"
	"gorm.io/gorm"
)

type GenerateMetadata struct {
	base.BaseTask
	App               *core.AppServer
	DeepSeekClient    *DeepSeekClient
	SavedVideoService *services.SavedVideoService
}

func NewGenerateMetadata(name string, app *core.AppServer, stateManager *manager.StateManager, client *cos.CosClient, apiKey string, db *gorm.DB, savedVideoService *services.SavedVideoService) *GenerateMetadata {
	return &GenerateMetadata{
		BaseTask: base.BaseTask{
			Name:         name,
			StateManager: stateManager,
			Client:       client,
		},
		App:               app,
		DeepSeekClient:    nil, // 不再固化客户端，运行时动态创建
		SavedVideoService: savedVideoService,
	}
}

// getCurrentDeepSeekClient 获取当前的DeepSeek客户端（使用最新配置）
func (g *GenerateMetadata) getCurrentDeepSeekClient() (*DeepSeekClient, error) {
	if g.App.Config.DeepSeekTransConfig == nil || !g.App.Config.DeepSeekTransConfig.Enabled {
		return nil, fmt.Errorf("DeepSeek 翻译服务未启用")
	}

	apiKey := g.App.Config.DeepSeekTransConfig.ApiKey
	if apiKey == "" {
		return nil, fmt.Errorf("DeepSeek API Key 未配置")
	}

	return NewDeepSeekClient(apiKey), nil
}

type VideoMetadata struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

func (g *GenerateMetadata) Execute(context map[string]interface{}) bool {
	g.App.Logger.Info("========================================")
	g.App.Logger.Infof("开始生成视频标题和描述: VideoID=%s", g.StateManager.VideoID)
	g.App.Logger.Info("========================================")

	// 0. 动态获取最新的DeepSeek客户端
	client, err := g.getCurrentDeepSeekClient()
	if err != nil {
		g.App.Logger.Errorf("❌ %v", err)
		// 使用默认值而不是失败
		context["video_title"] = g.StateManager.VideoID
		context["video_description"] = "包含字幕的视频"
		return true
	}

	g.App.Logger.Infof("🔑 使用最新的DeepSeek配置生成元数据")
	// 更新当前使用的客户端
	g.DeepSeekClient = client

	// 1. 检查中文字幕文件是否存在
	zhSRTPath := filepath.Join(g.StateManager.CurrentDir, "zh.srt")
	if _, err := os.Stat(zhSRTPath); os.IsNotExist(err) {
		g.App.Logger.Warn("⚠️  中文字幕文件不存在，使用默认标题和描述")
		// 使用默认值
		context["video_title"] = g.StateManager.VideoID
		context["video_description"] = fmt.Sprintf("包含字幕的视频")
		return true // 没有字幕文件不算失败
	}

	// 2. 读取中文字幕内容
	srtContent, err := os.ReadFile(zhSRTPath)
	if err != nil {
		g.App.Logger.Errorf("❌ 读取中文字幕文件失败: %v", err)
		context["error"] = "读取翻译字幕失败，请确保字幕翻译步骤已完成"
		return false
	}

	// 3. 解析字幕提取文本
	subtitleText := g.extractTextFromSRT(string(srtContent))
	if subtitleText == "" {
		g.App.Logger.Warn("⚠️  字幕内容为空，使用默认标题和描述")
		context["video_title"] = g.StateManager.VideoID
		context["video_description"] = fmt.Sprintf("包含字幕的视频")
		return true
	}

	g.App.Logger.Infof("📝 提取到字幕文本，总长度: %d 字符", len(subtitleText))

	// 4. 截取前1000字符用于生成标题和描述（避免token过多）
	maxLength := 1000
	if len(subtitleText) > maxLength {
		subtitleText = subtitleText[:maxLength] + "..."
	}

	// 5. 调用 DeepSeek API 生成标题和描述
	g.App.Logger.Info("🤖 调用 DeepSeek API 生成标题和描述...")
	metadata, err := g.generateMetadataFromDeepSeek(subtitleText)
	if err != nil {
		g.App.Logger.Errorf("❌ 生成标题和描述失败: %v", err)
		g.App.Logger.Warn("⚠️  将使用默认标题和描述，不影响视频上传")
		// 使用默认值
		context["video_title"] = g.StateManager.VideoID
		context["video_description"] = fmt.Sprintf("包含字幕的视频")
		return true // API调用失败不算整个任务失败
	}

	// 6. 验证标题长度（Bilibili限制80字符）
	if len([]rune(metadata.Title)) > 80 {
		runes := []rune(metadata.Title)
		metadata.Title = string(runes[:77]) + "..."
		g.App.Logger.Warnf("⚠️  标题过长，已截断为80字符")
	}

	// 7. 保存到 context
	context["video_title"] = metadata.Title
	context["video_description"] = metadata.Description
	context["video_tags"] = metadata.Tags

	// 8. 保存到 meta.json 文件
	g.App.Logger.Info("💾 保存元数据到 meta.json 文件...")
	if err := g.saveMetadataToFile(metadata); err != nil {
		g.App.Logger.Errorf("❌ 保存 meta.json 文件失败: %v", err)
		// 不影响任务继续执行
	} else {
		g.App.Logger.Info("✅ meta.json 文件已保存")
	}

	// 9. 保存到数据库
	g.App.Logger.Info("💾 保存生成的元数据到数据库...")
	savedVideo, err := g.SavedVideoService.GetVideoByVideoID(g.StateManager.VideoID)
	if err != nil {
		g.App.Logger.Errorf("❌ 获取视频记录失败: %v", err)
		// 不影响任务继续执行
	} else {
		// 更新生成的元数据
		savedVideo.GeneratedTitle = metadata.Title
		savedVideo.GeneratedDesc = metadata.Description
		savedVideo.GeneratedTags = strings.Join(metadata.Tags, ",")

		if err := g.SavedVideoService.UpdateVideo(savedVideo); err != nil {
			g.App.Logger.Errorf("❌ 保存元数据到数据库失败: %v", err)
		} else {
			g.App.Logger.Info("✅ 元数据已保存到数据库")
		}
	}

	// 10. 输出生成结果
	g.App.Logger.Info("========================================")
	g.App.Logger.Info("✅ 视频元数据生成成功！")
	g.App.Logger.Infof("📌 标题: %s", metadata.Title)
	g.App.Logger.Infof("📝 描述: %s", g.truncateString(metadata.Description, 100))
	g.App.Logger.Infof("🏷️  标签: %v", metadata.Tags)
	g.App.Logger.Info("========================================")

	return true
}

// extractTextFromSRT 从SRT内容中提取纯文本
func (g *GenerateMetadata) extractTextFromSRT(srtContent string) string {
	lines := strings.Split(srtContent, "\n")
	var textLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// 跳过空行、序号行、时间码行
		if line == "" || isNumber(line) || strings.Contains(line, "-->") {
			continue
		}
		textLines = append(textLines, line)
	}

	return strings.Join(textLines, " ")
}

// isNumber 检查字符串是否为数字
func isNumber(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

// generateMetadataFromDeepSeek 调用 DeepSeek API 生成标题和描述
func (g *GenerateMetadata) generateMetadataFromDeepSeek(subtitleText string) (*VideoMetadata, error) {
	prompt := fmt.Sprintf(`请根据以下视频字幕内容，生成一个吸引人的视频标题、详细描述和3-5个相关标签。

字幕内容：
%s

要求：
1. 标题要简洁有力，严格控制在30个字以内（B站限制80字，但建议30字以内更易读），能够准确概括视频主题，吸引观众点击
2. 描述要详细但不要过长，严格控制在600-800字以内，包含视频的主要内容和亮点（注意：B站简介限制2000字，需要预留约200字给原视频链接和分隔线）
3. 标签要准确反映视频内容，3-5个即可
4. 必须使用中文
5. 输出格式必须是JSON，格式如下：
{
  "title": "视频标题",
  "description": "视频描述",
  "tags": ["标签1", "标签2", "标签3"]
}

请直接返回JSON格式的结果，不要包含任何其他说明文字。`, subtitleText)

	// 使用 DeepSeekClient 调用 API
	content, usage, err := g.DeepSeekClient.ChatCompletionWithUsage("你是一个专业的视频内容分析助手，擅长根据视频字幕生成吸引人的标题和描述。", prompt)
	if err != nil {
		return nil, fmt.Errorf("调用 DeepSeek API 失败: %v", err)
	}

	g.App.Logger.Debugf("DeepSeek 原始返回: %s", content)

	// 提取JSON部分（可能包含在代码块中）
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
	var metadata VideoMetadata
	if err := json.Unmarshal([]byte(content), &metadata); err != nil {
		return nil, fmt.Errorf("解析元数据JSON失败: %v, 内容: %s", err, content)
	}

	// 验证必填字段
	if metadata.Title == "" {
		return nil, fmt.Errorf("生成的标题为空")
	}

	// Token使用情况
	if usage != nil {
		g.App.Logger.Infof("💰 Token使用: 输入=%d, 输出=%d, 总计=%d",
			usage.PromptTokens,
			usage.CompletionTokens,
			usage.TotalTokens)
	}

	return &metadata, nil
}

// saveMetadataToFile 保存元数据到 meta.json 文件
func (g *GenerateMetadata) saveMetadataToFile(metadata *VideoMetadata) error {
	// 构建文件路径
	metaFilePath := filepath.Join(g.StateManager.CurrentDir, "meta.json")

	// 创建一个包含更多信息的元数据结构
	fileMetadata := map[string]interface{}{
		"video_id":     g.StateManager.VideoID,
		"title":        metadata.Title,
		"description":  metadata.Description,
		"tags":         metadata.Tags,
		"generated_at": time.Now().Format("2006-01-02 15:04:05"),
	}

	// 转换为格式化的JSON
	jsonData, err := json.MarshalIndent(fileMetadata, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化元数据失败: %v", err)
	}

	// 写入文件
	if err := os.WriteFile(metaFilePath, jsonData, 0644); err != nil {
		return fmt.Errorf("写入meta.json文件失败: %v", err)
	}

	g.App.Logger.Infof("📁 meta.json 文件已保存: %s", metaFilePath)
	return nil
}

// truncateString 截断字符串用于日志显示
func (g *GenerateMetadata) truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

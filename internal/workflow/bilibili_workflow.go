package workflow

import (
	"context"
	"fmt"
	"strings"

	"github.com/zolagz/ytb2bili/internal/config"
	"github.com/zolagz/ytb2bili/pkg/llm"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// ============================================================================
// Bilibili 上传工作流 - 独立的任务链
// ============================================================================

// BilibiliWorkflowModule 提供 Bilibili 视频上传工作流的所有组件
var BilibiliWorkflowModule = fx.Module("bilibili_workflow",
	// 从 LLM 配置构造 LLM 客户端（可选；失败时仍可启动）

	// 提供元数据生成步骤和上传步骤
	fx.Options(StepProvidersForGroup("bilibili_steps",
		NewGenerateMetadataStep,
		NewUploadToBilibiliStep,
	)...),

	// 提供 Bilibili 任务链
	fx.Provide(NewBilibiliChain),
)

// NewWorkflowLLMClient 从 LLM 配置构造工作流使用的 LLM 客户端。
// 依次尝试 [chat] → [llm] 配置；API Key 未配置时返回 nil。
func NewWorkflowLLMClient(cfg *config.AppConfig, logger *zap.Logger) (*llm.EinoChatClient, error) {
	p := cfg.ResolveChatProvider()
	if p == nil || !p.ToLLMConfig().IsValid() {
		logger.Warn("⚠️  未配置 LLM，元数据自动生成将被跳过（请在 config.toml 中设置 [chat] 或 [llm] api_key）")
		return nil, nil
	}
	client, err := llm.NewClientFromConfig(p.ToLLMConfig(), logger)
	if err != nil {
		return nil, fmt.Errorf("workflow LLM client init failed: %w", err)
	}
	return client, nil
}

// ============================================================================
// Bilibili Context 定义
// ============================================================================

// BilibiliContext B站上传的上下文
type BilibiliContext struct {
	VideoContext               // 继承基础视频上下文
	BiliBVID            string // B站视频BVID
	BiliAID             int64  // B站视频AID
	UserID              string // 用户ID
	SubmissionOverrides *BilibiliSubmissionOverrides
}

type BilibiliSubmissionOverrides struct {
	AccountID   uint
	Copyright   int
	Title       string
	Description string
	Tags        string
	Cover       string
}

type bilibiliSubmissionCopyrightContextKey struct{}

// ============================================================================
// Bilibili 任务链
// ============================================================================

// BilibiliChain B站上传任务链
type BilibiliChain struct {
	metadataStep *GenerateMetadataStep
	uploadStep   *UploadToBilibiliStep
	logger       *zap.Logger
}

// BilibiliChainParams B站链的依赖参数
type BilibiliChainParams struct {
	fx.In
	Steps  []Step `group:"bilibili_steps"`
	Logger *zap.Logger
}

// NewBilibiliChain 创建B站上传任务链
func NewBilibiliChain(params BilibiliChainParams) *BilibiliChain {
	chain := &BilibiliChain{
		logger: params.Logger,
	}

	// 从steps中找到对应的步骤
	for _, step := range params.Steps {
		switch s := step.(type) {
		case *GenerateMetadataStep:
			chain.metadataStep = s
		case *UploadToBilibiliStep:
			chain.uploadStep = s
		}
	}

	return chain
}

// Run 执行B站上传流程
// input: *BilibiliContext 包含已下载的视频信息
func (bc *BilibiliChain) Run(ctx context.Context, input *BilibiliContext) error {
	bc.logger.Info("=== 开始B站上传工作流 ===",
		zap.String("video_id", input.VideoID),
		zap.String("user_id", input.UserID))

	// 将用户ID添加到context中（workflow 标准 key + 兼容旧 key）
	ctx = WithUserID(ctx, input.UserID)
	ctx = context.WithValue(ctx, "user_id", input.UserID)

	// 步骤1: 生成视频元数据（标题、描述、标签）
	if bc.metadataStep != nil {
		bc.logger.Info("📝 步骤1: 生成视频元数据")
		output, err := bc.metadataStep.Execute(ctx, &input.VideoContext)
		if err != nil {
			bc.logger.Warn("⚠️  元数据生成失败，将使用默认值", zap.Error(err))
			// 不中断流程，继续上传
		} else if vctx, ok := output.(*VideoContext); ok {
			input.VideoContext = *vctx
			bc.logger.Info("✅ 元数据生成成功",
				zap.String("title", vctx.Title),
				zap.String("tags", vctx.Tags))
			}
	} else {
		bc.logger.Warn("⚠️  未配置元数据生成步骤，跳过")
	}

	if input.SubmissionOverrides != nil {
		if input.SubmissionOverrides.AccountID > 0 {
			input.VideoContext.BiliAccountID = input.SubmissionOverrides.AccountID
		}
		if input.SubmissionOverrides.Copyright > 0 {
			ctx = context.WithValue(ctx, bilibiliSubmissionCopyrightContextKey{}, input.SubmissionOverrides.Copyright)
		}
		if strings.TrimSpace(input.SubmissionOverrides.Title) != "" {
			input.VideoContext.Title = strings.TrimSpace(input.SubmissionOverrides.Title)
		}
		if strings.TrimSpace(input.SubmissionOverrides.Description) != "" {
			input.VideoContext.Description = strings.TrimSpace(input.SubmissionOverrides.Description)
		}
		if strings.TrimSpace(input.SubmissionOverrides.Tags) != "" {
			input.VideoContext.Tags = strings.TrimSpace(input.SubmissionOverrides.Tags)
		}
		if strings.TrimSpace(input.SubmissionOverrides.Cover) != "" {
			input.VideoContext.ThumbnailPath = strings.TrimSpace(input.SubmissionOverrides.Cover)
		}
	}

	// 步骤2: 上传视频到B站
	bc.logger.Info("⬆️  步骤2: 上传视频到B站")
	output, err := bc.uploadStep.Execute(ctx, &input.VideoContext)
	if err != nil {
		bc.logger.Error("❌ B站上传失败", zap.Error(err))
		return err
	}
	// 更新context
	if vctx, ok := output.(*VideoContext); ok {
		input.BiliBVID = vctx.BiliBVID
		input.BiliAID = vctx.BiliAID
	}

	bc.logger.Info("=== B站上传工作流完成 ===",
		zap.String("bvid", input.BiliBVID),
		zap.Int64("aid", input.BiliAID))

	return nil
}

// RunFromVideoPath 从视频路径直接执行B站上传
// 简化接口，无需手动构建 BilibiliContext
func (bc *BilibiliChain) RunFromVideoPath(ctx context.Context, userID string, videoPath string, videoURL string, overrides *BilibiliSubmissionOverrides) (*BilibiliContext, error) {
	bctx := &BilibiliContext{
		VideoContext: VideoContext{
			VideoPath: videoPath,
			VideoURL:  videoURL,
			VideoID:   extractVideoID(videoPath),
		},
		UserID:              userID,
		SubmissionOverrides: overrides,
	}

	err := bc.Run(ctx, bctx)
	return bctx, err
}

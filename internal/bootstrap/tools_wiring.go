package bootstrap

import (
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/zolagz/ytb2bili/internal/config"
	"github.com/zolagz/ytb2bili/pkg/llm"
	"github.com/zolagz/ytb2bili/pkg/tikhub"
	agenttools "github.com/zolagz/ytb2bili/pkg/tools"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ToolsModule wires pkg/tools constructors with internal/config and internal/service.
var ToolsModule = fx.Module("tools",
	// ── LLM 客户端 ──────────────────────────────────────────────────
	// 只提供 Chat LLM 客户端（供 RewriteMetadataTool / SubtitleTool 等使用）。
	// 翻译用 LLM 由 workflow 模块的 provideLLMBatchTranslatorTool 内部创建。
	fx.Provide(provideChatLLMClient),

	// ── 工具 ────────────────────────────────────────────────────────
	fx.Provide(provideTools),
	fx.Provide(provideDBTools),
)

type toolsResult struct {
	fx.Out
	Tools []tool.BaseTool `group:"tools,flatten"`
}

// ── LLM Client Providers ──────────────────────────────────────────────────────


// provideChatLLMClient 提供对话/生成专用的 LLM 客户端。
// 配置来自 [chat] 或回退到 [llm]。
func provideChatLLMClient(cfg *config.AppConfig, logger *zap.Logger) (*llm.EinoChatClient, error) {
	p := cfg.ResolveChatProvider()
	if p == nil || !p.ToLLMConfig().IsValid() {
		return nil, fmt.Errorf("对话 LLM 未配置：请在 config.toml 中设置 [chat] 或 [llm] api_key")
	}
	return llm.NewClientFromConfig(p.ToLLMConfig(), logger)
}

// provideTools 提供需要 LLM 的常规工具。
func provideTools(cfg *config.AppConfig, logger *zap.Logger) (toolsResult, error) {
	reg := agenttools.NewToolRegistry()

	if t, err := agenttools.NewDownloadVideoTool(agenttools.DownloadVideoConfig{
		DownloadDir: cfg.Workflow.DownloadDir,
		YtDlpPath:   cfg.Workflow.YtDlpPath,
		CookiesFile: cfg.Workflow.CookiesFile,
		ProxyURL:    cfg.Workflow.ProxyURL,
	}, logger); err == nil {
		reg.MustRegister(logger, t, "download_video")
	} else {
		logger.Warn("download_video tool unavailable", zap.Error(err))
	}

	if t, err := agenttools.NewExtractAudioTool(cfg.Workflow.FFmpegPath, logger); err == nil {
		reg.MustRegister(logger, t, "extract_audio")
	} else {
		logger.Warn("extract_audio tool unavailable", zap.Error(err))
	}

	if t, err := agenttools.NewTranscodeVideoTool(cfg.Workflow.FFmpegPath, logger); err == nil {
		reg.MustRegister(logger, t, "transcode_video")
	} else {
		logger.Warn("transcode_video tool unavailable", zap.Error(err))
	}

	if t, err := agenttools.NewDownloadThumbnailTool(cfg.Workflow.DownloadDir, logger); err == nil {
		reg.MustRegister(logger, t, "download_thumbnail")
	} else {
		logger.Warn("download_thumbnail tool unavailable", zap.Error(err))
	}

	directResolver := tikhub.NewDirectResolver(logger)
	fetchTool := agenttools.NewFetchVideoByShareURLTool(directResolver, logger)
	reg.MustRegister(logger, fetchTool, "fetch_one_video_by_share_url")

	if t, err := agenttools.NewDownloadDouyinVideoTool(cfg.Workflow.DownloadDir, logger); err == nil {
		t.SetResolver(directResolver)
		reg.MustRegister(logger, t, "download_douyin_video")
	} else {
		logger.Warn("download_douyin_video tool unavailable", zap.Error(err))
	}

	return toolsResult{Tools: reg.All()}, nil
}

// provideDBTools 提供需要数据库访问的工具。
func provideDBTools(db *gorm.DB, chatLLM *llm.EinoChatClient, cfg *config.AppConfig, logger *zap.Logger) (toolsResult, error) {
	reg := agenttools.NewToolRegistry()

	reg.MustRegister(logger, agenttools.NewVideoQueryTool(db, logger), "query_videos")
	reg.MustRegister(logger, agenttools.NewSubscriptionTool(db, logger), "manage_subscription")

	if cfg.Server.Port > 0 {
		reg.MustRegister(logger, agenttools.NewSubmitPipelineTool(cfg.Server.Port, logger), "submit_pipeline")
	}

	reg.MustRegister(logger,
		agenttools.NewRewriteMetadataTool(db, chatLLM, nil, logger),
		"rewrite_metadata")
	reg.MustRegister(logger,
		agenttools.NewSubtitleTool(db, chatLLM, nil, cfg.Workflow.DownloadDir, logger),
		"subtitle_action")

	return toolsResult{Tools: reg.All()}, nil
}

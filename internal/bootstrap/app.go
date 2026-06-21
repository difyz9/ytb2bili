package bootstrap

import (
	"context"

	"github.com/zolagz/ytb2bili/internal/analytics"
	"github.com/zolagz/ytb2bili/internal/background"
	"github.com/zolagz/ytb2bili/internal/config"
	"github.com/zolagz/ytb2bili/internal/handler"
	"github.com/zolagz/ytb2bili/internal/server"
	"github.com/zolagz/ytb2bili/internal/service"
	"github.com/zolagz/ytb2bili/internal/updater"
	"github.com/zolagz/ytb2bili/internal/workflow"
	agent "github.com/zolagz/ytb2bili/pkg/agent"
	"github.com/zolagz/ytb2bili/pkg/store"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// NewApp wires fx modules and runs the app.
func NewApp() *fx.App {
	return fx.New(
		fx.Provide(zap.NewDevelopment),
		fx.Provide(config.LoadAppConfig),
		fx.Provide(config.AgenticConfig),
		fx.Provide(newUpdater),            // 自动更新管理器
		fx.Provide(func(cfg *config.AppConfig) bool { return cfg.IsLLMEnabled() }), // LLM 开关状态

		ToolsModule,  // 提供 tools（group:"tools,flatten"）
		agent.Module, // 消耗 tools group，提供 *agent.NanoAgent

		analytics.Module, // Analytics 数据统计模块
		store.Module,
		service.Module, // 业务服务模块
		background.Module,

		workflow.YouTubeWorkflowModule,  // YouTube 工作流模块
		workflow.DouyinWorkflowModule,   // 抖音工作流模块
		workflow.BilibiliWorkflowModule, // Bilibili 上传工作流模块

		// 视频处理编排服务（依赖 workflow chains）
		fx.Provide(workflow.NewProcessingService),

		handler.Module, // 先注册路由
		server.Module,  // 后启动服务器

		fx.Invoke(start),
	)
}

// start logs application startup/shutdown events.
func start(lc fx.Lifecycle, a *agent.NanoAgent, logger *zap.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			agentName := "(not configured)"
			if a != nil {
				agentName = a.Name
			}
			logger.Info("Application started", zap.String("agent", agentName))
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("Application stopped")
			return nil
		},
	})
}

// newUpdater creates an auto-update manager.
func newUpdater(appCfg *config.AppConfig, logger *zap.Logger) *updater.Updater {
	if !appCfg.Updater.Enabled {
		logger.Info("auto-update disabled")
		return nil
	}
	return updater.New(logger, &appCfg.Updater, appCfg.Workflow.YtDlpPath)
}

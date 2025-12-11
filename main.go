package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/difyz9/ytb2bili/internal/core"
	"github.com/difyz9/ytb2bili/internal/core/services"
	"github.com/difyz9/ytb2bili/internal/core/types"
	"github.com/difyz9/ytb2bili/internal/handler"
	"github.com/difyz9/ytb2bili/internal/web"
	"github.com/difyz9/ytb2bili/pkg/analytics"
	"github.com/difyz9/ytb2bili/pkg/cos"
	"github.com/difyz9/ytb2bili/pkg/logger"
	"github.com/difyz9/ytb2bili/pkg/store"
	"github.com/difyz9/ytb2bili/pkg/utils"
	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// AppLifecycle 应用程序生命周期
type AppLifecycle struct {
}

// OnStart 应用程序启动时执行
func (l *AppLifecycle) OnStart(context.Context) error {
	log.Println("AppLifecycle OnStart")
	return nil
}

// OnStop 应用程序停止时执行
func (l *AppLifecycle) OnStop(context.Context) error {
	log.Println("AppLifecycle OnStop")
	return nil
}

func main() {

	configFile := os.Getenv("CONFIG_FILE")
	if configFile == "" {
		configFile = "config.toml"
	}

	// 加载配置
	config, err := types.LoadConfig(configFile)
	if err != nil {
		log.Fatal(err)
	}
	config.Path = configFile

	app := fx.New(
		// 初始化配置应用配置
		fx.Provide(func() *types.AppConfig {
			return config
		}),

		// 日志模块
		fx.Provide(func(config *types.AppConfig) (*zap.SugaredLogger, error) {
			return logger.NewLogger(config.Debug)
		}),

		// 数据库模块
		fx.Provide(store.NewDatabase),

		// 核心模块
		fx.Provide(core.NewServer),
		fx.Provide(cos.NewCosClient),

		// 分析客户端
		fx.Provide(func(config *types.AppConfig, logger *zap.SugaredLogger) (*analytics.Client, error) {
			if config.AnalyticsConfig == nil || !config.AnalyticsConfig.Enabled {
				logger.Info("Analytics is disabled")
				return nil, nil
			}

			analyticsConfig := &analytics.Config{
				ServerURL:     config.AnalyticsConfig.ServerURL,
				APIKey:        config.AnalyticsConfig.APIKey,
				ProductID:     config.AnalyticsConfig.ProductID,
				Debug:         config.AnalyticsConfig.Debug,
				EncryptionKey: config.AnalyticsConfig.EncryptionKey,
			}

			return analytics.NewClient(analyticsConfig, logger)
		}),

		// 分析中间件
		fx.Provide(func(client *analytics.Client, logger *zap.SugaredLogger) *analytics.Middleware {
			return analytics.NewMiddleware(client, logger)
		}),

		// 服务层
		fx.Provide(services.NewVideoService),
		fx.Provide(services.NewSavedVideoService),
		fx.Provide(services.NewTaskStepService),

		// 注册cron
		fx.Provide(func() *cron.Cron {
			return cron.New(cron.WithSeconds())
		}),

		fx.Provide(handler.NewCronHandler),
		fx.Invoke(func(h *handler.CronHandler) {
			h.SetUp()
		}),

		// 生命周期管理
		fx.Provide(func() *AppLifecycle {
			return &AppLifecycle{}
		}),

		// 初始化数据库
		fx.Invoke(func(db *gorm.DB, logger *zap.SugaredLogger) error {
			logger.Info("Running database migrations...")
			return store.MigrateDatabase(db)
		}),

		// 初始化并检查 yt-dlp
		fx.Invoke(func(logger *zap.SugaredLogger, config *types.AppConfig) error {
			logger.Info("Checking yt-dlp installation...")
			return checkYtDlpInstallation(logger, config)
		}),

		// 初始化应用服务器和基础路由
		fx.Invoke(func(
			server *core.AppServer,
			db *gorm.DB,
			logger *zap.SugaredLogger,
			savedVideoService *services.SavedVideoService,
			taskStepService *services.TaskStepService,
			analyticsMiddleware *analytics.Middleware,
			analyticsClient *analytics.Client,
		) {
			// 初始化服务器
			server.Init(db)

			// 添加分析中间件
			if analyticsMiddleware != nil {
				server.Engine.Use(analyticsMiddleware.Handler())
				logger.Info("Analytics middleware registered")
			}

			// 注册所有 Handler 路由（包括连接 VideoHandler 和 UploadScheduler）
			registerHandlers(server, logger, savedVideoService, taskStepService, analyticsClient)

			// 健康检查
			server.Engine.GET("/health", func(c *gin.Context) {
				c.JSON(200, gin.H{
					"status":  "ok",
					"message": "Bili Up Backend API is running",
					"time":    time.Now().Format(time.RFC3339),
				})
			})

			// 静态文件服务 (嵌入的前端文件)
			logger.Info("Setting up embedded static file server...")
			staticHandler := web.StaticFileHandler()

			// 对于根路径和非 API 路径，提供静态文件
			server.Engine.NoRoute(func(c *gin.Context) {
				path := c.Request.URL.Path
				// 如果不是 API 路径，提供静态文件
				if !strings.HasPrefix(path, "/api/") && !strings.HasPrefix(path, "/health") {
					staticHandler.ServeHTTP(c.Writer, c.Request)
					return
				}
				// 否则返回 404
				c.JSON(404, gin.H{
					"code":    404,
					"message": "API endpoint not found",
				})
			})

			logger.Info("✓ Static file server configured")

		}),
		fx.Invoke(func(s *core.AppServer, db *gorm.DB) {
			go func() {
				err := s.Run()
				if err != nil {
					os.Exit(0)
				}
			}()
		}),
		// 注册生命周期回调函数
		fx.Invoke(func(lifecycle fx.Lifecycle, lc *AppLifecycle) {
			lifecycle.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					return lc.OnStart(ctx)
				},
				OnStop: func(ctx context.Context) error {
					return lc.OnStop(ctx)
				},
			})
		}),
	)

	// 启动应用程序
	go func() {

		if err := app.Start(context.Background()); err != nil {
			log.Fatal(err)
		}

	}()

	// 监听退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down gracefully...")

	// 关闭应用程序
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := app.Stop(ctx); err != nil {
		log.Fatal(err)
	}

	log.Println("✅ Application stopped")

}

// registerHandlers 注册所有 Handler 路由
func registerHandlers(
	server *core.AppServer,
	logger *zap.SugaredLogger,
	savedVideoService *services.SavedVideoService,
	taskStepService *services.TaskStepService,
	analyticsClient *analytics.Client,
) {
	logger.Info("Registering handlers...")

	// 认证 Handler
	authHandler := handler.NewAuthHandler(server)
	authHandler.RegisterRoutes(server)
	logger.Info("✓ Auth routes registered")

	// 上传 Handler
	uploadHandler := handler.NewUploadHandler(server)
	uploadHandler.RegisterRoutes(server)
	logger.Info("✓ Upload routes registered")

	// 分类 Handler
	categoryHandler := handler.NewCategoryHandler(server)
	categoryHandler.RegisterRoutes(server)
	logger.Info("✓ Category routes registered")

	// 字幕 Handler
	subtitleHandler := handler.NewSubtitleHandler(server)
	subtitleHandler.RegisterRoutes(server)
	logger.Info("✓ Subtitle routes registered")

	// 分析 Handler
	analyticsHandler := handler.NewAnalyticsHandler(analyticsClient, logger)

	// 视频 Handler
	videoHandler := handler.NewVideoHandler(server, savedVideoService, taskStepService)
	// 设置分析处理器
	videoHandler.AnalyticsHandler = analyticsHandler
	// 设置上传调度器（避免循环依赖）
	videoHandler.RegisterRoutes(server.Engine.Group("/api/v1"))
	logger.Info("✓ Video routes registered")

	// 配置 Handler
	configHandler := handler.NewConfigHandler(server)
	configHandler.RegisterRoutes(server)
	logger.Info("✓ Config routes registered")

	logger.Info("All handlers registered successfully")
}

// checkYtDlpInstallation 检查并自动安装 yt-dlp
func checkYtDlpInstallation(logger *zap.SugaredLogger, config *types.AppConfig) error {
	// 从配置中获取安装目录，如果未配置则使用默认值
	var installDir string
	if config != nil && config.YtDlpPath != "" {
		installDir = config.YtDlpPath
	}

	// 创建 yt-dlp 管理器
	manager := utils.NewYtDlpManager(logger, installDir)

	// 检查并自动安装
	if err := manager.CheckAndInstall(); err != nil {
		logger.Errorf("❌ yt-dlp 检查/安装失败: %v", err)
		logger.Warn("⚠️  视频下载功能可能无法正常工作")
		logger.Info("💡 您可以手动安装 yt-dlp:")
		logger.Info("   macOS: brew install yt-dlp")
		logger.Info("   Windows: winget install yt-dlp")
		logger.Info("   Linux: pip install yt-dlp")
		return nil // 不阻止应用启动
	}

	// 验证安装
	if err := manager.Validate(); err != nil {
		logger.Errorf("❌ yt-dlp 验证失败: %v", err)
		return nil // 不阻止应用启动
	}

	logger.Infof("✅ yt-dlp 就绪，路径: %s", manager.GetBinaryPath())
	return nil
}

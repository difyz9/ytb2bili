package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zolagz/ytb2bili/internal/config"
	"github.com/zolagz/ytb2bili/internal/middleware"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Module("server",
	fx.Provide(NewEngine),
	fx.Provide(NewHTTPServer),
	fx.Invoke(Start), // 启动服务器
)

func NewEngine(cfg *config.AppConfig) *gin.Engine {
	if cfg.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())

	// 配置静态文件服务 - 用于访问视频、字幕等文件
	if cfg.Server.StaticDir != "" {
		r.Static(cfg.Server.StaticPath, cfg.Server.StaticDir)
	}

	return r
}

func NewHTTPServer(cfg *config.AppConfig, r *gin.Engine) *http.Server {
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	return &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}
}

func Start(lc fx.Lifecycle, srv *http.Server, logger *zap.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Info("http server starting", zap.String("addr", srv.Addr))
			go func() {
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					logger.Error("http server error", zap.Error(err))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("http server stopping")
			return srv.Shutdown(ctx)
		},
	})

	logger.Info("http server wired", zap.String("addr", srv.Addr))
}

// SetupStaticFiles 配置前端静态文件服务
func SetupStaticFiles(r *gin.Engine, cfg *config.AppConfig, logger *zap.Logger) {
	logger.Info("🔧 配置静态文件服务", zap.Bool("debug", cfg.Debug))
	// 开发模式：前端独立运行（npm run dev）
	// 生产模式：使用 embed 嵌入的静态文件
	isDev := cfg.Debug
	ServeStaticWeb(r, logger, isDev)
}

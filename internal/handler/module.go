package handler

import (
	"github.com/zolagz/ytb2bili/internal/config"
	biliaccount "github.com/zolagz/ytb2bili/pkg/bilibili"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Module owns HTTP handlers and route registration.
var Module = fx.Module("handler",
	// ── Handler constructors ───────────────────────────────────────────────
	fx.Provide(NewHealthHandler),
	fx.Provide(NewVideoHandler),
	fx.Provide(NewAgentHandler),
	fx.Provide(NewAgentOpenHandler),
	fx.Provide(NewActivationHandler),
	fx.Provide(NewSubtitleHandler),
	fx.Provide(NewSwaggerHandler),
	fx.Provide(NewYouTubeHandler),
	fx.Provide(NewBiliAccountHandler),
	fx.Provide(NewTranslateHandler),
	fx.Provide(NewLocalAuthHandler),
	fx.Provide(NewUserHandler),
	fx.Provide(NewSystemSettingsHandler),
	fx.Provide(NewUserSettingsHandler),
	fx.Provide(NewAccountBindingHandler),
	fx.Provide(NewCookiesHandler),
	fx.Provide(NewUploadToBilibiliHandler),
	fx.Provide(NewVideoProcessHandler),
	fx.Provide(NewUpdaterHandler),
	fx.Provide(NewFeishuHandler),
	fx.Provide(NewTTSHandler),

	// ── Service dependencies consumed only by handlers ────────────────────
	fx.Provide(func(db *gorm.DB, logger *zap.Logger, cfg *config.AppConfig) *biliaccount.Service {
		options := biliaccount.Options{}
		if cfg != nil {
			options.CredentialsDir = cfg.Workflow.CredentialsDir
		}
		return biliaccount.NewService(db, logger, options)
	}),

	// ── Single route-wiring invocation ────────────────────────────────────
	fx.Invoke(registerRoutes),
)

// Package handler contains all HTTP handlers and centralized route registration.
package handler

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zolagz/ytb2bili/internal/config"
	"github.com/zolagz/ytb2bili/internal/middleware"
	"github.com/zolagz/ytb2bili/internal/server"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// RouterParams collects every handler through fx.In so that a single
// registerRoutes invocation can wire all routes onto the Gin engine.
//
// This pattern (borrowed from nanobot-go) replaces the previous approach of
// individual fx.Invoke(func(r *gin.Engine, h *XHandler){h.RegisterRoutes(r)})
// calls per handler, keeping module.go lean and route setup in one place.
type RouterParams struct {
	fx.In

	Engine *gin.Engine
	Config *config.AppConfig
	Logger *zap.Logger

	// Handlers (alphabetical)
	AccountBinding   *AccountBindingHandler
	Activation       *ActivationHandler
	Agent            *AgentHandler
	AgentOpen        *AgentOpenHandler
	BiliAccount      *BiliAccountHandler
	LocalAuth        *LocalAuthHandler
	Cookies          *CookiesHandler
	TTS              *TTSHandler
	Health           *HealthHandler
	Subtitle         *SubtitleHandler
	Swagger          *SwaggerHandler
	Translate        *TranslateHandler
	UploadToBilibili *UploadToBilibiliHandler
	Updater          *UpdaterHandler
	User             *UserHandler
	SystemSettings   *SystemSettingsHandler
	UserSettings     *UserSettingsHandler
	Video            *VideoHandler
	VideoProcess     *VideoProcessHandler
	YouTube          *YouTubeHandler
	Feishu           *FeishuHandler
}

// registerRoutes is the single fx.Invoke entry-point for all HTTP route
// registration.  It replaces the ~20 individual fx.Invoke calls that were
// previously scattered through module.go.
func registerRoutes(p RouterParams) {
	r := p.Engine

	var authMid gin.HandlerFunc
	localAuthSecret := strings.TrimSpace(p.Config.Auth.JWTSecret)
	localAuthEnabled := len(p.Config.Auth.Users) > 0
	if localAuthEnabled {
		authMid = middleware.AnyAuthMiddleware(localAuthSecret)
	} else if p.Config.APIAuth.Enabled &&
		p.Config.APIAuth.AppID != "" &&
		p.Config.APIAuth.AppSecret != "" {
		authMid = middleware.NewAuthMiddleware(middleware.AuthConfig{
			AppID:             p.Config.APIAuth.AppID,
			AppSecret:         p.Config.APIAuth.AppSecret,
			CookiesDecryptKey: p.Config.APIAuth.CookiesDecryptKey,
		})
	}

	// Core handlers
	p.Health.RegisterRoutes(r)
	p.Agent.RegisterRoutes(r)
	p.Swagger.RegisterRoutes(r)
	p.LocalAuth.RegisterRoutes(r)
	p.Activation.RegisterRoutes(r)

	// SubtitleHandler may need an optional decrypt middleware for meta cookies.
	// This should work in both local-auth mode and api_auth mode.
	var decryptMid gin.HandlerFunc
	if strings.TrimSpace(p.Config.APIAuth.CookiesDecryptKey) != "" {
		decryptMid = middleware.DecryptCookies(p.Config.APIAuth.CookiesDecryptKey)
	}
	p.Subtitle.RegisterRoutesWithAuth(r, decryptMid)

	p.AgentOpen.RegisterRoutes(r)

	// Feature handlers
	p.YouTube.RegisterRoutes(r)
	p.BiliAccount.RegisterRoutesWithAuth(r, authMid)
	p.User.RegisterRoutes(r)
	p.SystemSettings.RegisterRoutes(r)
	p.UserSettings.RegisterRoutes(r)
	p.AccountBinding.RegisterRoutes(r)
	p.Cookies.RegisterRoutes(r)
	p.UploadToBilibili.RegisterRoutes(r)
	p.Updater.RegisterRoutes(r)
	p.Video.RegisterRoutesWithAuth(r, authMid)
	p.VideoProcess.RegisterRoutesWithAuth(r, authMid)
	{
		translateGroup := r.Group("/api/v1/translate")
		translateGroup.POST("/subtitles", p.Translate.TranslateSubtitles)
	}

	// Optional channels
	p.Feishu.RegisterRoutes(r)

	p.TTS.RegisterRoutes(r)

	// Static / SPA fallback must be registered last (NoRoute catch-all)
	server.ServeStaticWeb(r, p.Logger, p.Config.Debug)
}

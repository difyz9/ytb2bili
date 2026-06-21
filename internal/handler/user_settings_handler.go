package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zolagz/ytb2bili/internal/config"
	"github.com/zolagz/ytb2bili/internal/middleware"
	"github.com/zolagz/ytb2bili/internal/service"
	biliaccount "github.com/zolagz/ytb2bili/pkg/bilibili"
	"go.uber.org/zap"
)

type UserSettingsHandler struct {
	settings  *service.UserSettingsClient
	accounts  *biliaccount.Service
	logger    *zap.Logger
	jwtSecret string
	authMid   gin.HandlerFunc
}

const userSettingsDBTimeout = 5 * time.Second

func NewUserSettingsHandler(settings *service.UserSettingsClient, accounts *biliaccount.Service, logger *zap.Logger, cfg *config.AppConfig) *UserSettingsHandler {
	var authMid gin.HandlerFunc
	localAuthEnabled := cfg != nil && len(cfg.Auth.Users) > 0
	if !localAuthEnabled && cfg != nil && cfg.APIAuth.Enabled && cfg.APIAuth.AppID != "" && cfg.APIAuth.AppSecret != "" {
		authMid = middleware.NewAuthMiddleware(middleware.AuthConfig{
			AppID:             cfg.APIAuth.AppID,
			AppSecret:         cfg.APIAuth.AppSecret,
			CookiesDecryptKey: cfg.APIAuth.CookiesDecryptKey,
		})
	}

	jwtSecret := ""
	if cfg != nil {
		jwtSecret = strings.TrimSpace(cfg.Auth.JWTSecret)
	}

	return &UserSettingsHandler{
		settings:  settings,
		accounts:  accounts,
		logger:    logger,
		jwtSecret: jwtSecret,
		authMid:   authMid,
	}
}

func (h *UserSettingsHandler) GetSettings(c *gin.Context) {
	uid := c.GetString("uid")
	ctx, cancel := context.WithTimeout(context.Background(), userSettingsDBTimeout)
	defer cancel()

	settings, err := h.settings.GetSettings(ctx, uid)
	if err != nil {
		h.logger.Error("读取用户设置失败", zap.String("uid", uid), zap.Error(err))
		InternalServerError(c, "读取用户设置失败")
		return
	}

	Success(c, gin.H{"settings": settings})
}

func (h *UserSettingsHandler) UpdateSettings(c *gin.Context) {
	uid := c.GetString("uid")
	var patch map[string]string
	if err := c.ShouldBindJSON(&patch); err != nil {
		BadRequest(c, "请求体必须为 JSON 对象")
		return
	}
	if len(patch) == 0 {
		BadRequest(c, "无有效的设置项可更新")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), userSettingsDBTimeout)
	defer cancel()

	updated, err := h.settings.UpdateSettings(ctx, uid, patch)
	if err != nil {
		h.logger.Warn("更新用户设置失败", zap.String("uid", uid), zap.Error(err))
		var updateErr *service.UserSettingsUpdateError
		if errors.As(err, &updateErr) {
			switch updateErr.StatusCode {
			case http.StatusUnauthorized:
				Unauthorized(c, updateErr.Message)
			case http.StatusForbidden:
				Forbidden(c, updateErr.Message)
			case http.StatusServiceUnavailable:
				ServiceUnavailable(c, updateErr.Message)
			default:
				BadRequest(c, updateErr.Message)
			}
			return
		}
		BadRequest(c, fmt.Sprintf("更新用户设置失败: %v", err))
		return
	}

	Success(c, gin.H{"settings": updated.ToSettingsMap()})
}

func (h *UserSettingsHandler) GetBilibiliVideoZones(c *gin.Context) {
	uid := c.GetString("uid")
	if uid == "" {
		Unauthorized(c, "未获取到用户身份")
		return
	}
	if h.accounts == nil {
		InternalServerError(c, "B站账号服务未配置")
		return
	}

	zones, err := h.accounts.ListVideoZones(c.Request.Context(), uid)
	if err != nil {
		h.logger.Warn("加载B站投稿分区失败", zap.String("uid", uid), zap.Error(err))
		InternalServerError(c, "获取投稿分区失败")
		return
	}

	Success(c, gin.H{"zones": zones})
}

func (h *UserSettingsHandler) RegisterRoutes(r *gin.Engine) {
	anyAuth := middleware.AnyAuthMiddleware(h.jwtSecret)
	for _, basePath := range []string{"/api/v1/user/settings", "/api/user/settings", "/user/settings"} {
		group := r.Group(basePath)
		if h.authMid != nil {
			group.Use(h.authMid)
		}
		group.Use(anyAuth)
		{
			group.GET("/bilibili-video-zones", h.GetBilibiliVideoZones)
			group.GET("", h.GetSettings)
			group.PUT("", h.UpdateSettings)
		}
	}
}

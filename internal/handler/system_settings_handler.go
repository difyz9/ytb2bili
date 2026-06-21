package handler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zolagz/ytb2bili/internal/config"
	"github.com/zolagz/ytb2bili/internal/middleware"
	"github.com/zolagz/ytb2bili/internal/service"
	"go.uber.org/zap"
)

type SystemSettingsHandler struct {
	settings  *service.SystemSettingsClient
	logger    *zap.Logger
	jwtSecret string
}

const systemSettingsDBTimeout = 5 * time.Second

func NewSystemSettingsHandler(settings *service.SystemSettingsClient, logger *zap.Logger, cfg *config.AppConfig) *SystemSettingsHandler {
	jwtSecret := ""
	if cfg != nil {
		jwtSecret = strings.TrimSpace(cfg.Auth.JWTSecret)
	}

	return &SystemSettingsHandler{
		settings:  settings,
		logger:    logger,
		jwtSecret: jwtSecret,
	}
}

func (h *SystemSettingsHandler) GetSettings(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), systemSettingsDBTimeout)
	defer cancel()

	settings, err := h.settings.GetSettings(ctx)
	if err != nil {
		h.logger.Error("读取系统设置失败", zap.Error(err))
		InternalServerError(c, "读取系统设置失败")
		return
	}

	Success(c, gin.H{"settings": settings})
}

func (h *SystemSettingsHandler) UpdateSettings(c *gin.Context) {
	var patch map[string]string
	if err := c.ShouldBindJSON(&patch); err != nil {
		BadRequest(c, "请求体必须为 JSON 对象")
		return
	}
	if len(patch) == 0 {
		BadRequest(c, "无有效的设置项可更新")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), systemSettingsDBTimeout)
	defer cancel()

	updated, err := h.settings.UpdateSettings(ctx, patch)
	if err != nil {
		h.logger.Warn("更新系统设置失败", zap.Error(err))
		BadRequest(c, fmt.Sprintf("更新系统设置失败: %v", err))
		return
	}

	Success(c, gin.H{"settings": updated.ToSettingsMap()})
}

func (h *SystemSettingsHandler) RegisterRoutes(r *gin.Engine) {
	anyAuth := middleware.AnyAuthMiddleware(h.jwtSecret)
	for _, basePath := range []string{"/api/v1/system/settings", "/api/system/settings", "/system/settings"} {
		group := r.Group(basePath)
		group.Use(anyAuth)
		{
			group.GET("", h.GetSettings)
			group.PUT("", h.UpdateSettings)
		}
	}
}

package handler

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zolagz/ytb2bili/internal/updater"
	"go.uber.org/zap"
)

// UpdaterHandler 自动更新处理器
type UpdaterHandler struct {
	updater *updater.Updater
	logger  *zap.Logger
}

// NewUpdaterHandler 创建更新处理器
func NewUpdaterHandler(upd *updater.Updater, logger *zap.Logger) *UpdaterHandler {
	return &UpdaterHandler{
		updater: upd,
		logger:  logger,
	}
}

// GetVersion 获取当前版本信息
// @Summary 获取当前版本
// @Description 获取当前应用版本号和构建信息
// @Tags Updater
// @Accept json
// @Produce json
// @Success 200 {object} Response{data=object}
// @Router /api/v1/updater/version [get]
func (h *UpdaterHandler) GetVersion(c *gin.Context) {
	if h.updater == nil {
		Success(c, gin.H{
			"version": "unknown",
			"enabled": false,
			"message": "自动更新功能未启用",
		})
		return
	}

	Success(c, gin.H{
		"version":             h.updater.GetCurrentVersion(),
		"enabled":             true,
		"autoUpdate":          h.updater.IsAutoUpdateEnabled(),
		"restartOnSuccess":    h.updater.IsRestartOnSuccessEnabled(),
		"restartDelaySeconds": int(h.updater.RestartDelay() / time.Second),
	})
}

// CheckUpdate 检查更新
// @Summary 检查是否有新版本
// @Description 检查 GitHub Releases 是否有新版本可用
// @Tags Updater
// @Accept json
// @Produce json
// @Success 200 {object} Response{data=object}
// @Failure 500 {object} Response
// @Router /api/v1/updater/check [post]
func (h *UpdaterHandler) CheckUpdate(c *gin.Context) {
	if h.updater == nil {
		BadRequest(c, "自动更新功能未启用")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	h.logger.Info("API: 检查更新")
	h.updater.TriggerYtDlpBackgroundUpdate()

	hasUpdate, latestVersion, err := h.updater.CheckForUpdates(ctx)
	if err != nil {
		h.logger.Error("检查更新失败", zap.Error(err))
		InternalServerError(c, "检查更新失败: "+err.Error())
		return
	}

	Success(c, gin.H{
		"hasUpdate":      hasUpdate,
		"currentVersion": h.updater.GetCurrentVersion(),
		"latestVersion":  latestVersion,
		"message":        getMessage(hasUpdate),
	})
}

// DoUpdate 执行更新
// @Summary 执行更新
// @Description 下载并安装新版本（公开接口，建议在生产环境中添加权限控制）
// @Tags Updater
// @Accept json
// @Produce json
// @Success 200 {object} Response{data=object}
// @Failure 500 {object} Response
// @Router /api/v1/updater/update [post]
func (h *UpdaterHandler) DoUpdate(c *gin.Context) {
	if h.updater == nil {
		BadRequest(c, "自动更新功能未启用")
		return
	}

	// 注意：生产环境建议添加管理员权限检查
	// 示例: if !isAdmin(c) { Forbidden(c, "需要管理员权限"); return }

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	h.logger.Info("API: 执行更新",
		zap.String("current_version", h.updater.GetCurrentVersion()))

	if err := h.updater.ValidateUpdateEnvironment(); err != nil {
		BadRequest(c, err.Error())
		return
	}

	// 先检查是否有更新
	hasUpdate, latestVersion, err := h.updater.CheckForUpdates(ctx)
	if err != nil {
		h.logger.Error("检查更新失败", zap.Error(err))
		InternalServerError(c, "检查更新失败: "+err.Error())
		return
	}

	if !hasUpdate {
		Success(c, gin.H{
			"updated":        false,
			"currentVersion": h.updater.GetCurrentVersion(),
			"message":        "已是最新版本，无需更新",
		})
		return
	}

	Success(c, gin.H{
		"updated":        false,
		"started":        true,
		"currentVersion": h.updater.GetCurrentVersion(),
		"latestVersion":  latestVersion,
		"message":        "更新任务已启动，请稍候查看进度",
		"needRestart":    false,
	})

	go func(latest string) {
		updateCtx, updateCancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer updateCancel()

		if runErr := h.updater.DoUpdate(updateCtx); runErr != nil {
			h.logger.Error("更新失败", zap.Error(runErr), zap.String("target_version", latest))
			return
		}

		h.logger.Info("更新成功",
			zap.String("to_version", latest),
			zap.String("current_version", h.updater.GetCurrentVersion()))
	}(latestVersion)
}

// GetUpdateStatus 获取更新状态
// @Summary 获取更新状态
// @Description 获取自动更新配置和状态信息
// @Tags Updater
// @Accept json
// @Produce json
// @Success 200 {object} Response{data=object}
// @Router /api/v1/updater/status [get]
func (h *UpdaterHandler) GetUpdateStatus(c *gin.Context) {
	if h.updater == nil {
		Success(c, gin.H{
			"enabled":        false,
			"autoUpdate":     false,
			"currentVersion": "unknown",
			"message":        "自动更新功能未启用",
		})
		return
	}

	Success(c, gin.H{
		"enabled":             true,
		"autoUpdate":          h.updater.IsAutoUpdateEnabled(),
		"restartOnSuccess":    h.updater.IsRestartOnSuccessEnabled(),
		"restartDelaySeconds": int(h.updater.RestartDelay() / time.Second),
		"currentVersion":      h.updater.GetCurrentVersion(),
		"updating":            h.updater.GetUpdateStatus().Updating,
		"progress":            h.updater.GetUpdateStatus().Progress,
		"latestVersion":       h.updater.GetUpdateStatus().LatestVersion,
		"message":             h.updater.GetUpdateStatus().Message,
		"lastCheckedAt":       h.updater.GetUpdateStatus().LastCheckedAt,
	})
}

// RegisterRoutes 实现 RouteRegistrar 接口，将更新相关路由注册到引擎。
func (h *UpdaterHandler) RegisterRoutes(r *gin.Engine) {
	RegisterUpdaterRoutes(r.Group("/api/v1"), h)
}

// RegisterUpdaterRoutes 注册更新相关路由
func RegisterUpdaterRoutes(r *gin.RouterGroup, handler *UpdaterHandler) {
	updater := r.Group("/updater")
	{
		// 所有接口均为公开，无需认证
		updater.GET("/version", handler.GetVersion)
		updater.POST("/check", handler.CheckUpdate)
		updater.POST("/update", handler.DoUpdate) // 注意：生产环境建议添加权限控制
		updater.GET("/status", handler.GetUpdateStatus)
	}
}

// 辅助函数

func getMessage(hasUpdate bool) string {
	if hasUpdate {
		return "发现新版本，可以更新"
	}
	return "当前已是最新版本"
}

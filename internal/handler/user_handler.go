package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zolagz/ytb2bili/internal/config"
	"github.com/zolagz/ytb2bili/internal/middleware"
	"github.com/zolagz/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// UserHandler 用户处理器
type UserHandler struct {
	db        *gorm.DB
	logger    *zap.Logger
	jwtSecret string
}

// NewUserHandler 创建用户处理器
func NewUserHandler(db *gorm.DB, logger *zap.Logger, cfg *config.AppConfig) *UserHandler {
	jwtSecret := ""
	if cfg != nil {
		jwtSecret = strings.TrimSpace(cfg.Auth.JWTSecret)
	}

	return &UserHandler{
		db:        db,
		logger:    logger,
		jwtSecret: jwtSecret,
	}
}

// GetCurrentUser godoc
// @Summary 获取当前登录用户信息
// @Tags user
// @Produce json
// @Security BearerAuth
// @Success 200 {object} UserInfoResponse
// @Failure 401 {object} map[string]interface{} "error: string"
// @Router /api/user/me [get]
func (h *UserHandler) GetCurrentUser(c *gin.Context) {
	uid, _ := c.Get("uid")
	email, _ := c.Get("email")
	provider, _ := c.Get("provider")

	uidStr, _ := uid.(string)
	emailStr, _ := email.(string)
	providerStr, _ := provider.(string)

	displayName := emailStr
	if idx := strings.Index(emailStr, "@"); idx > 0 {
		displayName = emailStr[:idx]
	}
	c.JSON(http.StatusOK, UserInfoResponse{
		ID:          uidStr,
		DisplayName: displayName,
		Email:       emailStr,
		PhotoURL:    "",
		Provider:    providerStr,
	})
}

// UpdateUserInfoRequest 更新用户信息请求
type UpdateUserInfoRequest struct {
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
}

// UpdateUserInfo godoc
// @Summary 更新用户信息
// @Tags user
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body UpdateUserInfoRequest true "用户信息"
// @Success 200 {object} UserInfoResponse "更新后的用户信息"
// @Failure 400 {object} map[string]interface{} "error: string"
// @Failure 401 {object} map[string]interface{} "error: string"
// @Router /api/user/update [put]
func (h *UserHandler) UpdateUserInfo(c *gin.Context) {
	uid, exists := c.Get("uid")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	var req UpdateUserInfoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user model.User
	if err := h.db.Where("firebase_uid = ?", uid).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	if req.Username != "" {
		user.Username = req.Username
	}
	if req.Avatar != "" {
		user.Avatar = req.Avatar
	}

	if err := h.db.Save(&user).Error; err != nil {
		h.logger.Error("更新用户信息失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}

	displayName := user.Username
	if displayName == "" {
		parts := strings.SplitN(user.Email, "@", 2)
		if len(parts) > 0 && parts[0] != "" {
			displayName = parts[0]
		} else {
			displayName = "用户"
		}
	}

	c.JSON(http.StatusOK, UserInfoResponse{
		ID:          user.FirebaseUID,
		DisplayName: displayName,
		Email:       user.Email,
		PhotoURL:    user.Avatar,
		Provider:    "email",
	})
}

// RegisterRoutes 注册路由
func (h *UserHandler) RegisterRoutes(r *gin.Engine) {
	anyAuth := middleware.AnyAuthMiddleware(h.jwtSecret)

	userGroup := r.Group("/api/user")
	userGroup.Use(anyAuth)
	{
		userGroup.GET("/me", h.GetCurrentUser)
		userGroup.GET("/info", h.GetCurrentUser)
		userGroup.PUT("/update", h.UpdateUserInfo)
	}
}

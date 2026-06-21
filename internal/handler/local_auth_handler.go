package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zolagz/ytb2bili/internal/config"
	"github.com/zolagz/ytb2bili/internal/middleware"
	"go.uber.org/zap"
)

type LocalAuthHandler struct {
	cfg    *config.AppConfig
	logger *zap.Logger
}

type localAuthLoginRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type localAuthUserResponse struct {
	ID     string  `json:"id"`
	Email  string  `json:"email"`
	Name   string  `json:"name"`
	Avatar *string `json:"avatar"`
	Role   string  `json:"role"`
}

func NewLocalAuthHandler(cfg *config.AppConfig, logger *zap.Logger) *LocalAuthHandler {
	return &LocalAuthHandler{cfg: cfg, logger: logger}
}

func (h *LocalAuthHandler) RegisterRoutes(r *gin.Engine) {
	auth := r.Group("/auth")
	{
		auth.POST("/login", h.login)
		auth.POST("/register", h.registerDisabled)
		auth.POST("/refresh", h.refresh)
		auth.POST("/logout", h.logout)
		auth.GET("/me", middleware.AnyAuthMiddleware(h.jwtSecret()), h.me)
	}
}

func (h *LocalAuthHandler) login(c *gin.Context) {
	var req localAuthLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "请求参数错误")
		return
	}

	user, ok := h.matchUser(req)
	if !ok {
		Unauthorized(c, "用户名或密码错误")
		return
	}

	accessToken, refreshToken, expiresIn, err := h.issueTokenPair(user)
	if err != nil {
		h.logger.Error("签发登录令牌失败", zap.Error(err))
		InternalServerError(c, "登录失败")
		return
	}

	Success(c, gin.H{
		"user": localAuthUserResponse{
			ID:     userID(*user),
			Email:  userEmail(*user),
			Name:   userDisplayName(*user),
			Avatar: userAvatar(*user),
			Role:   userRole(*user),
		},
		"accessToken":  accessToken,
		"refreshToken": refreshToken,
		"expiresIn":    int(expiresIn.Seconds()),
	})
}

func (h *LocalAuthHandler) refresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refreshToken"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "请求参数错误")
		return
	}

	secret := h.jwtSecret()
	if secret == "" {
		ServiceUnavailable(c, "登录服务未配置")
		return
	}

	claims, err := middleware.ParseLocalToken(strings.TrimSpace(req.RefreshToken), secret, "refresh")
	if err != nil {
		Unauthorized(c, "刷新令牌无效或已过期")
		return
	}

	user, ok := h.findUserByIDOrEmail(claims.UID, claims.Email)
	if !ok {
		Unauthorized(c, "用户不存在")
		return
	}

	accessToken, refreshToken, expiresIn, err := h.issueTokenPair(user)
	if err != nil {
		h.logger.Error("刷新登录令牌失败", zap.Error(err))
		InternalServerError(c, "刷新登录状态失败")
		return
	}

	Success(c, gin.H{
		"user": localAuthUserResponse{
			ID:     userID(*user),
			Email:  userEmail(*user),
			Name:   userDisplayName(*user),
			Avatar: userAvatar(*user),
			Role:   userRole(*user),
		},
		"accessToken":  accessToken,
		"refreshToken": refreshToken,
		"expiresIn":    int(expiresIn.Seconds()),
	})
}

func (h *LocalAuthHandler) me(c *gin.Context) {
	uid := strings.TrimSpace(c.GetString("uid"))
	email := strings.TrimSpace(c.GetString("email"))
	if uid == "" && email == "" {
		Unauthorized(c, "未登录")
		return
	}

	user, ok := h.findUserByIDOrEmail(uid, email)
	if !ok {
		Unauthorized(c, "用户不存在")
		return
	}

	Success(c, gin.H{
		"user": localAuthUserResponse{
			ID:     userID(*user),
			Email:  userEmail(*user),
			Name:   userDisplayName(*user),
			Avatar: userAvatar(*user),
			Role:   userRole(*user),
		},
	})
}

func (h *LocalAuthHandler) logout(c *gin.Context) {
	SuccessWithEmpty(c)
}

func (h *LocalAuthHandler) registerDisabled(c *gin.Context) {
	c.JSON(http.StatusForbidden, Response{
		Code:    http.StatusForbidden,
		Data:    EmptyData{},
		Message: "当前部署仅允许配置账号登录，注册功能已关闭",
	})
}

func (h *LocalAuthHandler) matchUser(req localAuthLoginRequest) (*config.LocalAuthUser, bool) {
	username := strings.TrimSpace(req.Username)
	email := strings.TrimSpace(req.Email)
	password := strings.TrimSpace(req.Password)
	if password == "" {
		return nil, false
	}

	for i := range h.cfg.Auth.Users {
		candidate := &h.cfg.Auth.Users[i]
		if strings.TrimSpace(candidate.Password) == "" {
			continue
		}
		if candidate.Password != password {
			continue
		}
		if username != "" && strings.EqualFold(strings.TrimSpace(candidate.Username), username) {
			return candidate, true
		}
		if email != "" && strings.EqualFold(strings.TrimSpace(candidate.Email), email) {
			return candidate, true
		}
	}

	if email != "" {
		for i := range h.cfg.Auth.Users {
			candidate := &h.cfg.Auth.Users[i]
			if strings.TrimSpace(candidate.Password) == "" {
				continue
			}
			if candidate.Password == password && strings.EqualFold(strings.TrimSpace(candidate.Username), email) {
				return candidate, true
			}
		}
	}

	return nil, false
}

func (h *LocalAuthHandler) findUserByIDOrEmail(uid, email string) (*config.LocalAuthUser, bool) {
	for i := range h.cfg.Auth.Users {
		candidate := &h.cfg.Auth.Users[i]
		if uid != "" && userID(*candidate) == uid {
			return candidate, true
		}
		if email != "" && strings.EqualFold(strings.TrimSpace(candidate.Email), email) {
			return candidate, true
		}
	}
	return nil, false
}

func (h *LocalAuthHandler) issueTokenPair(user *config.LocalAuthUser) (accessToken, refreshToken string, expiresIn time.Duration, err error) {
	accessTTL := time.Duration(h.cfg.Auth.AccessTokenTTL) * time.Second
	refreshTTL := time.Duration(h.cfg.Auth.RefreshTokenTTL) * time.Second
	if accessTTL <= 0 {
		accessTTL = 2 * time.Hour
	}
	if refreshTTL <= 0 {
		refreshTTL = 7 * 24 * time.Hour
	}

	accessToken, refreshToken, err = middleware.IssueLocalTokenPair(
		h.jwtSecret(),
		time.Now(),
		userID(*user),
		userEmail(*user),
		userDisplayName(*user),
		userRole(*user),
		accessTTL,
		refreshTTL,
	)
	if err != nil {
		return "", "", 0, err
	}

	return accessToken, refreshToken, accessTTL, nil
}

func (h *LocalAuthHandler) jwtSecret() string {
	if h.cfg == nil {
		return ""
	}
	return strings.TrimSpace(h.cfg.Auth.JWTSecret)
}

func userID(user config.LocalAuthUser) string {
	if strings.TrimSpace(user.ID) != "" {
		return strings.TrimSpace(user.ID)
	}
	if strings.TrimSpace(user.Email) != "" {
		return strings.TrimSpace(user.Email)
	}
	return strings.TrimSpace(user.Username)
}

func userEmail(user config.LocalAuthUser) string {
	if strings.TrimSpace(user.Email) != "" {
		return strings.TrimSpace(user.Email)
	}
	if strings.Contains(strings.TrimSpace(user.Username), "@") {
		return strings.TrimSpace(user.Username)
	}
	return strings.TrimSpace(user.Username) + "@local"
}

func userDisplayName(user config.LocalAuthUser) string {
	if strings.TrimSpace(user.DisplayName) != "" {
		return strings.TrimSpace(user.DisplayName)
	}
	if strings.TrimSpace(user.Username) != "" {
		return strings.TrimSpace(user.Username)
	}
	return strings.TrimSpace(user.Email)
}

func userRole(user config.LocalAuthUser) string {
	if strings.TrimSpace(user.Role) != "" {
		return strings.TrimSpace(user.Role)
	}
	return "admin"
}

func userAvatar(user config.LocalAuthUser) *string {
	avatar := strings.TrimSpace(user.Avatar)
	if avatar == "" {
		return nil
	}
	return &avatar
}

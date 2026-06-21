package handler

import (
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zolagz/ytb2bili/internal/config"
	"github.com/zolagz/ytb2bili/internal/service"
	"github.com/zolagz/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ── Handler ─────────────────────────────────────────────────────────────────

// ActivationHandler 处理激活码提交和验证
type ActivationHandler struct {
	cfg         *config.AppConfig
	client      *service.LicenseClient
	db          *gorm.DB
	logger      *zap.Logger
}

// NewActivationHandler 创建 ActivationHandler
func NewActivationHandler(
	cfg *config.AppConfig,
	client *service.LicenseClient,
	db *gorm.DB,
	logger *zap.Logger,
) *ActivationHandler {
	return &ActivationHandler{
		cfg:    cfg,
		client: client,
		db:     db,
		logger: logger,
	}
}

// RegisterRoutes 注册路由
func (h *ActivationHandler) RegisterRoutes(r *gin.Engine) {
	group := r.Group("/api/v1/activate")
	{
		group.POST("", h.activate)
		group.GET("/status", h.status)
	}
}

// ── Request / Response ───────────────────────────────────────────────────────

type activateRequest struct {
	LicenseKey string `json:"license_key" binding:"required"`
}

type activateResponse struct {
	Activated bool       `json:"activated"`
	Tier      string     `json:"tier"`
	Plan      string     `json:"plan"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Product   string     `json:"product,omitempty"`
	LicenseKey string    `json:"license_key,omitempty"`
}

type statusResponse struct {
	Configured bool       `json:"configured"`
	Activated  bool       `json:"activated"`
	Activation *struct {
		LicenseKey  string     `json:"license_key"`
		Tier        string     `json:"tier"`
		Plan        string     `json:"plan"`
		ActivatedAt time.Time  `json:"activated_at"`
		ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	} `json:"activation,omitempty"`
}

// ── Handlers ─────────────────────────────────────────────────────────────────

// activate 提交激活码进行验证
func (h *ActivationHandler) activate(c *gin.Context) {
	// 检查是否配置了验证服务
	if !h.client.IsConfigured() {
		ServiceUnavailable(c, "License 验证服务未配置，请联系管理员设置 [license] 配置项")
		return
	}

	var req activateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "请提供激活码 (license_key)")
		return
	}

	key := strings.TrimSpace(req.LicenseKey)
	if key == "" {
		BadRequest(c, "激活码不能为空")
		return
	}

	// 调用 Worker API 验证
	result, err := h.client.Verify(key)
	if err != nil {
		h.logger.Error("验证激活码失败", zap.String("key", maskKey(key)), zap.Error(err))
		ServiceUnavailable(c, "验证服务暂时不可用，请稍后重试")
		return
	}

	if !result.Valid {
		BadRequest(c, result.Message)
		return
	}

	// 验证通过 — 保存到数据库
	now := time.Now()
	activation := model.LicenseActivation{
		LicenseKey:  result.LicenseKey,
		UserID:      result.LicenseKey, // 用 LicenseKey 作为 UserID 标识
		Tier:        model.Tier(result.Tier),
		Plan:        result.Plan,
		ExpiresAt:   result.ExpiresAt,
		ActivatedAt: now,
	}

	// 事务：写入激活记录 + 更新用户会员信息
	txErr := h.db.Transaction(func(tx *gorm.DB) error {
		// 删除旧的同密钥激活记录（防止重复）
		tx.Where("license_key = ?", result.LicenseKey).Delete(&model.LicenseActivation{})

		if err := tx.Create(&activation).Error; err != nil {
			return err
		}

		// 更新/创建用户会员信息
		membership := model.UserMembership{
			UserID: result.LicenseKey,
			Tier:   model.Tier(result.Tier),
		}
		membership.CreatedAt = now
		membership.UpdatedAt = now
		if result.ExpiresAt != nil {
			membership.ExpiresAt = *result.ExpiresAt
		} else {
			// 无过期时间 = 永久有效
			membership.ExpiresAt = time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC)
		}

		// UPSERT: 存在则更新，不存在则创建
		tx.Where("user_id = ?", result.LicenseKey).Delete(&model.UserMembership{})
		if err := tx.Create(&membership).Error; err != nil {
			return err
		}

		return nil
	})

	if txErr != nil {
		h.logger.Error("保存激活信息失败", zap.Error(txErr))
		InternalServerError(c, "保存激活信息失败")
		return
	}

	Success(c, activateResponse{
		Activated:  true,
		Tier:       result.Tier,
		Plan:       result.Plan,
		ExpiresAt:  result.ExpiresAt,
		Product:    result.Product,
		LicenseKey: maskKey(result.LicenseKey),
	})
}

// status 查询当前激活状态
func (h *ActivationHandler) status(c *gin.Context) {
	resp := statusResponse{
		Configured: h.client.IsConfigured(),
		Activated:  false,
	}

	if !h.client.IsConfigured() {
		Success(c, resp)
		return
	}

	// 查找最近的一条激活记录
	var activation model.LicenseActivation
	err := h.db.Order("id DESC").First(&activation).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			Success(c, resp)
			return
		}
		h.logger.Error("查询激活状态失败", zap.Error(err))
		InternalServerError(c, "查询激活状态失败")
		return
	}

	// 检查是否过期
	now := time.Now()
	isExpired := activation.ExpiresAt != nil && activation.ExpiresAt.Before(now)
	if isExpired {
		resp.Activated = false
		Success(c, resp)
		return
	}

	// 查询对应的会员信息
	var membership model.UserMembership
	memErr := h.db.Where("user_id = ?", activation.UserID).First(&membership).Error
	if memErr != nil {
		if errors.Is(memErr, gorm.ErrRecordNotFound) {
			// 有激活记录但无会员记录，仍然显示已激活
			resp.Activated = true
			resp.Activation = &struct {
				LicenseKey  string     `json:"license_key"`
				Tier        string     `json:"tier"`
				Plan        string     `json:"plan"`
				ActivatedAt time.Time  `json:"activated_at"`
				ExpiresAt   *time.Time `json:"expires_at,omitempty"`
			}{
				LicenseKey:  maskKey(activation.LicenseKey),
				Tier:        string(activation.Tier),
				Plan:        activation.Plan,
				ActivatedAt: activation.ActivatedAt,
				ExpiresAt:   activation.ExpiresAt,
			}
			Success(c, resp)
			return
		}
		h.logger.Error("查询会员信息失败", zap.Error(memErr))
	}

	resp.Activated = true
	resp.Activation = &struct {
		LicenseKey  string     `json:"license_key"`
		Tier        string     `json:"tier"`
		Plan        string     `json:"plan"`
		ActivatedAt time.Time  `json:"activated_at"`
		ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	}{
		LicenseKey:  maskKey(activation.LicenseKey),
		Tier:        string(membership.Tier),
		Plan:        activation.Plan,
		ActivatedAt: activation.ActivatedAt,
		ExpiresAt:   activation.ExpiresAt,
	}

	Success(c, resp)
}

// maskKey 隐藏激活码中间部分，仅显示首尾
func maskKey(key string) string {
	if len(key) <= 12 {
		return key[:4] + "****" + key[len(key)-4:]
	}
	return key[:6] + "****" + key[len(key)-6:]
}

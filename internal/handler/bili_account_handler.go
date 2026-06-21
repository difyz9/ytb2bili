package handler

import (
	"fmt"
	"strconv"

	biliaccount "github.com/zolagz/ytb2bili/pkg/bilibili"
	"github.com/zolagz/ytb2bili/pkg/store/model"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// BiliAccountHandler B站账号处理器
type BiliAccountHandler struct {
	service *biliaccount.Service
	logger  *zap.Logger
}

// NewBiliAccountHandler 创建B站账号处理器
func NewBiliAccountHandler(service *biliaccount.Service, logger *zap.Logger) *BiliAccountHandler {
	return &BiliAccountHandler{
		service: service,
		logger:  logger,
	}
}

// RegisterRoutes 注册路由
func (h *BiliAccountHandler) RegisterRoutes(r *gin.Engine) {
	h.RegisterRoutesWithAuth(r, nil)
}

// RegisterRoutesWithAuth 注册路由并可选注入鉴权中间件
func (h *BiliAccountHandler) RegisterRoutesWithAuth(r *gin.Engine, authMid gin.HandlerFunc) {
	api := r.Group("/api/v1/bili-accounts")
	if authMid != nil {
		api.Use(authMid)
	}
	{
		api.GET("", h.listAccounts)             // 获取账号列表
		api.DELETE("/:id", h.unbindAccount)     // 解绑账号
		api.PUT("/:id/primary", h.setPrimary)   // 设置主账号
	}
}

// listAccounts godoc
// @Summary 获取B站账号列表
// @Description 获取当前用户绑定的所有B站账号
// @Tags bili-accounts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} Response{data=[]map[string]interface{}}
// @Failure 401 {object} Response "未授权"
// @Failure 500 {object} Response "服务器错误"
// @Router /api/v1/bili-accounts [get]
func (h *BiliAccountHandler) listAccounts(c *gin.Context) {
	userID, exists := c.Get("uid")
	if !exists {
		Unauthorized(c, "未授权")
		return
	}

	uid, ok := userID.(string)
	if !ok {
		Unauthorized(c, "用户ID类型错误")
		return
	}

	accounts, err := h.service.GetUserBiliAccounts(uid)
	if err != nil {
		h.logger.Error("获取账号列表失败", zap.Error(err), zap.String("user_id", uid))
		InternalServerError(c, "获取账号列表失败")
		return
	}

	// 转换为安全的返回格式
	safeAccounts := make([]map[string]interface{}, 0, len(accounts))
	for _, acc := range accounts {
		safeAccounts = append(safeAccounts, GetSafeAccount(&acc))
	}

	Success(c, safeAccounts)
}

// unbindAccount godoc
// @Summary 解绑B站账号
// @Description 解绑指定的B站账号
// @Tags bili-accounts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "账号ID"
// @Success 200 {object} Response
// @Failure 400 {object} Response "参数错误"
// @Failure 401 {object} Response "未授权"
// @Failure 404 {object} Response "账号不存在"
// @Failure 500 {object} Response "服务器错误"
// @Router /api/v1/bili-accounts/{id} [delete]
func (h *BiliAccountHandler) unbindAccount(c *gin.Context) {
	userID, exists := c.Get("uid")
	if !exists {
		Unauthorized(c, "未授权")
		return
	}

	uid, ok := userID.(string)
	if !ok {
		Unauthorized(c, "用户ID类型错误")
		return
	}

	accountID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "无效的账号ID")
		return
	}

	// 获取账号信息以验证权限和获取 bili_mid
	accounts, err := h.service.GetUserBiliAccounts(uid)
	if err != nil {
		h.logger.Error("获取账号失败", zap.Error(err))
		InternalServerError(c, "获取账号失败")
		return
	}

	var biliMid int64
	for _, acc := range accounts {
		if acc.ID == uint(accountID) {
			fmt.Sscanf(acc.PlatformUID, "%d", &biliMid)
			break
		}
	}

	if biliMid == 0 {
		NotFound(c, "账号不存在或无权限")
		return
	}

	// 删除账号
	if err := h.service.DeleteBiliAccount(uid, biliMid); err != nil {
		h.logger.Error("删除账号失败", zap.Error(err))
		InternalServerError(c, "删除账号失败")
		return
	}

	Success(c, nil)
}

// setPrimary godoc
// @Summary 设置主账号
// @Description 将指定账号设置为主账号
// @Tags bili-accounts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "账号ID"
// @Success 200 {object} Response
// @Failure 400 {object} Response "参数错误"
// @Failure 401 {object} Response "未授权"
// @Failure 404 {object} Response "账号不存在"
// @Failure 500 {object} Response "服务器错误"
// @Router /api/v1/bili-accounts/{id}/primary [put]
func (h *BiliAccountHandler) setPrimary(c *gin.Context) {
	userID, exists := c.Get("uid")
	if !exists {
		Unauthorized(c, "未授权")
		return
	}

	uid, ok := userID.(string)
	if !ok {
		Unauthorized(c, "用户ID类型错误")
		return
	}

	accountID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "无效的账号ID")
		return
	}

	// 获取账号信息以获取 bili_mid
	accounts, err := h.service.GetUserBiliAccounts(uid)
	if err != nil {
		h.logger.Error("获取账号失败", zap.Error(err))
		InternalServerError(c, "获取账号失败")
		return
	}

	var biliMid int64
	for _, acc := range accounts {
		if acc.ID == uint(accountID) {
			fmt.Sscanf(acc.PlatformUID, "%d", &biliMid)
			break
		}
	}

	if biliMid == 0 {
		NotFound(c, "账号不存在或无权限")
		return
	}

	// 设置主账号
	if err := h.service.SetPrimaryAccount(uid, biliMid); err != nil {
		h.logger.Error("设置主账号失败", zap.Error(err))
		InternalServerError(c, "设置主账号失败")
		return
	}

	Success(c, nil)
}

// GetSafeAccount 返回安全的账号信息（隐藏敏感字段）
func GetSafeAccount(account *model.AccountBinding) map[string]interface{} {
	biliData, _ := account.GetBiliData()
	var biliMid int64
	if biliData != nil {
		biliMid = biliData.BiliMid
	}
	return map[string]interface{}{
		"id":           account.ID,
		"bili_mid":     biliMid,
		"bili_name":    account.Username,
		"bili_face":    account.Avatar,
		"is_primary":   account.IsPrimary,
		"expires_at":   account.ExpiresAt,
		"last_used_at": account.LastUsedAt,
		"created_at":   account.CreatedAt,
		"updated_at":   account.UpdatedAt,
	}
}

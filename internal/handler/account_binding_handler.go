package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/difyz9/bilibili-go-sdk/bilibili"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zolagz/ytb2bili/internal/analytics"
	"github.com/zolagz/ytb2bili/internal/config"
	internalservice "github.com/zolagz/ytb2bili/internal/service"
	biliaccount "github.com/zolagz/ytb2bili/pkg/bilibili"
	"github.com/zolagz/ytb2bili/pkg/store"
	"github.com/zolagz/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

// newBilibiliClient 创建不走系统代理的B站客户端
// Go默认Transport会读取 HTTPS_PROXY 环境变量；B站API应直连，避免因本地代理未启动而报错。
func newBilibiliClient() *bilibili.Client {
	return bilibili.NewClient(
		bilibili.WithHTTPClient(&http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				// 显式置空 Proxy 函数，强制直连，不读取 HTTP_PROXY / HTTPS_PROXY 环境变量
				Proxy: func(*http.Request) (*url.URL, error) { return nil, nil },
			},
		}),
	)
}

// AccountBindingHandler 账号绑定处理器
type AccountBindingHandler struct {
	logger             *zap.Logger
	cfg                *config.AppConfig
	biliAccountService *biliaccount.Service
	bindingCache       *store.CacheDict
	analytics          *analytics.Client
	youtubeClient      *internalservice.YouTubeClientFactory
	youtubeBinding     *internalservice.YouTubeBindingService
	bindingService     *internalservice.BindingService
}

// NewAccountBindingHandler 创建账号绑定处理器
func NewAccountBindingHandler(
	logger *zap.Logger,
	cfg *config.AppConfig,
	biliAccountService *biliaccount.Service,
	analyticsClient *analytics.Client,
	youtubeClient *internalservice.YouTubeClientFactory,
	youtubeBinding *internalservice.YouTubeBindingService,
	bindingService *internalservice.BindingService,
) *AccountBindingHandler {
	cache := store.NewTempDict()
	if sharedBindingCache == nil {
		sharedBindingCache = cache
	}
	return &AccountBindingHandler{
		logger:             logger,
		cfg:                cfg,
		biliAccountService: biliAccountService,
		bindingCache:       cache,
		analytics:          analyticsClient,
		youtubeClient:      youtubeClient,
		youtubeBinding:     youtubeBinding,
		bindingService:     bindingService,
	}
}
// RegisterRoutes 注册路由
func (h *AccountBindingHandler) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api/v1/bindings")
	{
		api.POST("/qrcode", h.GenerateBindingQRCode)         // 生成绑定二维码（B站等扫码平台）
		api.POST("/poll", h.PollBindingStatus)               // 轮询绑定状态
		api.GET("/list", h.GetBindingList)                   // 获取绑定列表
		api.DELETE("/:id", h.UnbindAccount)                  // 解绑账号
		api.PUT("/:id/primary", h.SetPrimaryBinding)        // 设置主账号（B站）
		api.POST("/refresh", h.RefreshToken)                 // 刷新令牌
		api.GET("/youtube/authorize", h.YouTubeAuthorize)    // 获取YouTube OAuth授权URL（桌面应用方式）
		api.GET("/youtube/callback", h.YouTubeOAuthCallback) // YouTube OAuth回调
		api.POST("/youtube/complete", h.YouTubeCompleteAuthorization)
	}
}

// GenerateBindingQRCodeRequest 生成绑定二维码请求
type GenerateBindingQRCodeRequest struct {
	Platform string `json:"platform" binding:"required"` // 平台: bilibili, douyin, kuaishou
	UserID   string `json:"user_id" binding:"required"`  // 用户ID
}

// GenerateBindingQRCodeResponse 生成绑定二维码响应
type GenerateBindingQRCodeResponse struct {
	QRCode    string `json:"qr_code"`     // 二维码URL
	QRCodeKey string `json:"qr_code_key"` // 二维码密钥
	ExpiresIn int64  `json:"expires_in"`  // 过期时间（秒）
}

// GenerateBindingQRCode godoc
// @Summary 生成账号绑定二维码
// @Description 为指定平台生成账号绑定二维码，用户扫码后可绑定账号
// @Tags account-bindings
// @Accept json
// @Produce json
// @Param request body GenerateBindingQRCodeRequest true "生成二维码请求"
// @Success 200 {object} Response{data=GenerateBindingQRCodeResponse}
// @Failure 400 {object} Response "参数错误"
// @Failure 500 {object} Response "服务器错误"
// @Router /api/v1/bindings/qrcode [post]
func (h *AccountBindingHandler) GenerateBindingQRCode(c *gin.Context) {
	var req GenerateBindingQRCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数错误: "+err.Error())
		return
	}

	// 验证平台（YouTube使用独立的OAuth授权流程，不走二维码）
	validPlatforms := map[string]bool{
		"bilibili": true,
		"douyin":   true,
		"kuaishou": true,
	}
	if !validPlatforms[req.Platform] {
		BadRequest(c, "不支持的平台")
		return
	}

	if req.Platform != string(model.PlatformBilibili) {
		// 非 B 站平台沿用单账号绑定语义，避免改变其他尚未完善的平台逻辑。
		var existingBinding model.AccountBinding
		err := h.bindingService.GetDB().Where("user_id = ? AND platform = ? AND status = ?",
			req.UserID, req.Platform, model.BindingStatusBound).
			First(&existingBinding).Error

		if err == nil {
			BadRequest(c, fmt.Sprintf("该账号已绑定%s平台，请先解绑后再重新绑定", req.Platform))
			return
		} else if err != gorm.ErrRecordNotFound {
			h.logger.Error("检查已有绑定失败", zap.Error(err))
			InternalServerError(c, "数据库查询失败")
			return
		}
	}

	// 生成二维码key
	qrCodeKey := uuid.New().String()

	// 根据平台生成相应的二维码
	qrCode, authCode, err := h.generatePlatformQRCode(req.Platform, qrCodeKey)
	if err != nil {
		h.logger.Error("生成二维码失败", zap.Error(err))
		InternalServerError(c, "生成二维码失败")
		return
	}

	// 创建绑定记录
	binding := &model.AccountBinding{
		UserID:   req.UserID,
		Platform: model.Platform(req.Platform),

		Status: model.BindingStatusPending,
	}

	if err := h.bindingService.GetDB().Create(binding).Error; err != nil {
		h.logger.Error("保存绑定记录失败", zap.Error(err))
		InternalServerError(c, "数据库创建失败")
		return
	}

	// 将绑定信息存储到临时缓存中，设置5分钟过期
	bindingData := map[string]interface{}{
		"id":         binding.ID,
		"user_id":    binding.UserID,
		"platform":   binding.Platform,
		"qr_code":    qrCode, // 使用生成的二维码URL
		"status":     binding.Status,
		"created_at": time.Now().Unix(),
	}
	if authCode != "" {
		bindingData["auth_code"] = authCode
	}

	h.bindingCache.Set(qrCodeKey, bindingData, 5*time.Minute)

	Success(c, GenerateBindingQRCodeResponse{
		QRCode:    qrCode,
		QRCodeKey: qrCodeKey,
		ExpiresIn: 300,
	})
}

// generatePlatformQRCode 根据平台生成二维码
func (h *AccountBindingHandler) generatePlatformQRCode(platform, qrCodeKey string) (qrCodeURL string, authCode string, err error) {
	switch platform {
	case "bilibili":
		client := newBilibiliClient()
		qrResp, err := client.GetQRCode()
		if err != nil {
			return "", "", fmt.Errorf("获取B站二维码失败: %w", err)
		}
		if qrResp.Code != 0 {
			return "", "", fmt.Errorf("B站返回错误: code=%d", qrResp.Code)
		}
		return qrResp.Data.URL, qrResp.Data.AuthCode, nil

	case "douyin":
		// TODO: 实现抖音二维码生成
		return "", "", fmt.Errorf("抖音平台暂未实现")

	case "kuaishou":
		// TODO: 实现快手二维码生成
		return "", "", fmt.Errorf("快手平台暂未实现")

	default:
		return "", "", fmt.Errorf("不支持的平台: %s", platform)
	}
}

// PollBindingStatusRequest 轮询绑定状态请求
type PollBindingStatusRequest struct {
	QRCodeKey string `json:"qr_code_key" binding:"required"`
}

// PollBindingStatusResponse 轮询绑定状态响应
type PollBindingStatusResponse struct {
	Status      string `json:"status"`       // pending, bound, expired
	Platform    string `json:"platform"`     // 平台
	PlatformUID string `json:"platform_uid"` // 平台用户ID
	Username    string `json:"username"`     // 平台用户名
	Avatar      string `json:"avatar"`       // 平台头像
}

// PollBindingStatus godoc
// @Summary 轮询绑定状态
// @Description 轮询二维码扫描和绑定状态，用于前端实时检查绑定进度
// @Tags account-bindings
// @Accept json
// @Produce json
// @Param request body PollBindingStatusRequest true "轮询请求"
// @Success 200 {object} Response{data=PollBindingStatusResponse}
// @Failure 400 {object} Response "参数错误"
// @Failure 500 {object} Response "服务器错误"
// @Router /api/v1/bindings/poll [post]
func (h *AccountBindingHandler) PollBindingStatus(c *gin.Context) {
	var req PollBindingStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数错误: "+err.Error())
		return
	}

	// 从缓存中获取绑定信息
	var bindingData map[string]interface{}
	if err := h.bindingCache.Get(req.QRCodeKey, &bindingData); err != nil {
		// 缓存中不存在或已过期，直接返回过期状态
		Success(c, PollBindingStatusResponse{Status: "expired"})
		return
	}

	platform := fmt.Sprintf("%v", bindingData["platform"])

	// 根据平台轮询扫码状态
	switch platform {
	case "bilibili":
		if authCode, ok := bindingData["auth_code"].(string); ok && authCode != "" {
			loginData, err := h.pollBilibiliQRCode(authCode)
			if err != nil {
				h.logger.Debug("B站二维码未扫描", zap.Error(err))
				Success(c, PollBindingStatusResponse{Status: "pending", Platform: platform})
				return
			}

			if loginData != nil {
				// 扫码成功，保存绑定信息
				userID := fmt.Sprintf("%v", bindingData["user_id"])
				if err := h.saveBilibiliBinding(userID, loginData); err != nil {
					h.logger.Error("保存B站绑定失败", zap.Error(err))
					InternalServerError(c, "保存绑定失败")
					return
				}

				// 清理缓存
				h.bindingCache.Delete(req.QRCodeKey)

				// 从数据库读取刚保存的绑定信息（包含完整的用户信息）
				var savedBinding model.AccountBinding
				if err := h.bindingService.GetDB().Where("user_id = ? AND platform = ? AND platform_uid = ?",
					userID, model.PlatformBilibili, fmt.Sprintf("%d", loginData.TokenInfo.Mid)).
					First(&savedBinding).Error; err == nil {
					// 使用数据库中保存的完整信息
					Success(c, PollBindingStatusResponse{
						Status:      "bound",
						Platform:    platform,
						PlatformUID: savedBinding.PlatformUID,
						Username:    savedBinding.Username,
						Avatar:      savedBinding.Avatar,
					})
				} else {
					// 如果读取失败，使用登录信息中的基本数据
					Success(c, PollBindingStatusResponse{
						Status:      "bound",
						Platform:    platform,
						PlatformUID: fmt.Sprintf("%d", loginData.TokenInfo.Mid),
						Username:    loginData.TokenInfo.Uname,
						Avatar:      loginData.TokenInfo.Face,
					})
				}
				return
			}
		}

	}

	// 其他平台或未扫码，返回等待状态
	Success(c, PollBindingStatusResponse{
		Status:   "pending",
		Platform: platform,
	})
}

// pollBilibiliQRCode 轮询B站二维码状态
func (h *AccountBindingHandler) pollBilibiliQRCode(authCode string) (*bilibili.LoginInfo, error) {
	client := newBilibiliClient()
	loginInfo, err := client.PollQRCode(authCode)
	if err != nil {
		return nil, err
	}
	return loginInfo, nil
}

// saveBilibiliBinding 保存B站绑定
func (h *AccountBindingHandler) saveBilibiliBinding(userID string, loginData *bilibili.LoginInfo) error {
	if loginData == nil {
		return fmt.Errorf("登录数据无效")
	}

	tokenInfo := loginData.TokenInfo
	if tokenInfo.Mid == 0 {
		return fmt.Errorf("无效的用户Mid")
	}

	// 将完整 LoginInfo 序列化为 JSON 存入 cookies 字段，上传时可直接反序列化还原
	loginInfoJSON, err := json.Marshal(loginData)
	if err != nil {
		return fmt.Errorf("序列化登录信息失败: %w", err)
	}
	cookiesPlain := string(loginInfoJSON)

	// 加密敏感凭证后再持久化
	cookies, err := h.biliAccountService.Encrypt(cookiesPlain)
	if err != nil {
		return fmt.Errorf("加密cookies失败: %w", err)
	}
	accessTokenEnc, err := h.biliAccountService.Encrypt(tokenInfo.AccessToken)
	if err != nil {
		return fmt.Errorf("加密access_token失败: %w", err)
	}
	refreshTokenEnc, err := h.biliAccountService.Encrypt(tokenInfo.RefreshToken)
	if err != nil {
		return fmt.Errorf("加密refresh_token失败: %w", err)
	}

	// 计算过期时间
	var expiresAt *time.Time
	if tokenInfo.ExpiresIn > 0 {
		expiry := time.Now().Add(time.Duration(tokenInfo.ExpiresIn) * time.Second)
		expiresAt = &expiry
	}

	// 初始化用户信息（使用登录返回的基本信息）
	userName := tokenInfo.Uname
	userAvatar := tokenInfo.Face
	userLevel := 0
	userFans := 0
	userAttention := 0
	userCoins := 0
	userSign := ""
	userVip := false

	if userName == "" {
		userName = fmt.Sprintf("用户_%d", tokenInfo.Mid)
	}

	// 优先使用myinfo API获取最新的完整用户信息（参考 auth_handler.go）
	cookieString := loginData.GetCookieString()
	client := newBilibiliClient()
	myInfo, err := client.GetMyInfoWithRetry(cookieString, 2)
	if err == nil && myInfo != nil {
		// 使用myinfo API返回的完整信息
		userName = myInfo.Uname
		userAvatar = myInfo.Face
		userLevel = myInfo.Level
		userFans = myInfo.Fans
		userAttention = myInfo.Attention
		userCoins = myInfo.GetCoins()
		userSign = myInfo.Sign
		// 注意：MyInfoResponse中没有VIP字段，暂时设为false
		// TODO: 需要调用其他API获取VIP信息
		userVip = false

		// h.logger.Info("成功获取B站用户完整信息",
		// 	zap.String("username", userName),
		// 	zap.Int("level", userLevel),
		// 	zap.Int("fans", userFans),
		// 	zap.Int("attention", userAttention),
		// 	zap.Int("coins", userCoins),
		// 	zap.String("sign", userSign),
		// 	zap.Bool("vip", userVip))
	} else {
		h.logger.Warn("获取B站用户详细信息失败，使用登录返回的基本信息", zap.Error(err))
	}

	// 构建B站平台数据
	platformData := &model.BiliPlatformData{
		BiliMid:   tokenInfo.Mid,
		BiliLevel: userLevel,
		BiliVip:   userVip,
	}

	// 检查是否已存在绑定
	var existingBinding model.AccountBinding
	result := h.bindingService.GetDB().Where("user_id = ? AND platform = ? AND platform_uid = ?",
		userID, model.PlatformBilibili, fmt.Sprintf("%d", tokenInfo.Mid)).
		First(&existingBinding)

	now := time.Now()

	if result.Error == gorm.ErrRecordNotFound {
		// 创建新绑定（使用从myinfo获取的完整用户信息）
		binding := &model.AccountBinding{
			UserID:       userID,
			Platform:     model.PlatformBilibili,
			PlatformUID:  fmt.Sprintf("%d", tokenInfo.Mid),
			Username:     userName,   // 使用从API获取的用户名
			Avatar:       userAvatar, // 使用从API获取的头像
			AccessToken:  accessTokenEnc,
			RefreshToken: refreshTokenEnc,
			ExpiresAt:    expiresAt,
			Status:       model.BindingStatusBound,
			Cookies:      cookies,
			LastUsedAt:   &now,
		}

		// 设置平台数据
		if err := binding.SetBiliData(platformData); err != nil {
			return fmt.Errorf("设置B站数据失败: %w", err)
		}

		// 检查是否是第一个B站账号，如果是则设为主账号
		var count int64
		h.bindingService.GetDB().Model(&model.AccountBinding{}).
			Where("user_id = ? AND platform = ? AND status = ?",
				userID, model.PlatformBilibili, model.BindingStatusBound).
			Count(&count)
		binding.IsPrimary = (count == 0)

		if err := h.bindingService.GetDB().Create(binding).Error; err != nil {
			h.logger.Error("创建B站绑定失败", zap.Error(err))
			return fmt.Errorf("创建B站绑定失败: %w", err)
		}

		h.logger.Info("成功创建B站绑定",
			zap.String("user_id", userID),
			zap.Int64("bili_mid", tokenInfo.Mid),
			zap.String("username", userName))

		// 将凭证同步写入本地文件（上传时优先从此文件加载）
		if err := h.biliAccountService.SaveCredentialToFile(binding); err != nil {
			h.logger.Warn("保存B站凭证到本地文件失败", zap.Error(err))
		}

		h.trackBilibiliBindingSuccess(
			userID,
			fmt.Sprintf("%d", tokenInfo.Mid),
			userName,
			userAvatar,
			userLevel,
			userFans,
			userAttention,
			userCoins,
			userSign,
			userVip,
			binding.IsPrimary,
			"bind",
		)
		return nil
	} else if result.Error != nil {
		return fmt.Errorf("查询绑定记录失败: %w", result.Error)
	}

	// 更新现有绑定（使用从myinfo获取的完整用户信息）
	updates := map[string]interface{}{
		"username":      userName,   // 使用从API获取的用户名
		"avatar":        userAvatar, // 使用从API获取的头像
		"access_token":  accessTokenEnc,
		"refresh_token": refreshTokenEnc,
		"expires_at":    expiresAt,
		"status":        model.BindingStatusBound,
		"cookies":       cookies,
		"last_used_at":  &now,
	}

	// 更新平台数据
	existingBinding.SetBiliData(platformData)
	if existingBinding.PlatformData != nil {
		updates["platform_data"] = *existingBinding.PlatformData
	}

	if err := h.bindingService.GetDB().Model(&existingBinding).Updates(updates).Error; err != nil {
		h.logger.Error("更新B站绑定失败", zap.Error(err))
		return fmt.Errorf("更新B站绑定失败: %w", err)
	}

	h.logger.Info("成功更新B站绑定",
		zap.String("user_id", userID),
		zap.Int64("bili_mid", tokenInfo.Mid),
		zap.String("username", userName))

	// 将最新凭证同步写入本地文件（覆盖旧版本）
	// 从数据库重新读取使 PlatformData 等更新字段都已就位
	if err := h.biliAccountService.SaveCredentialToFile(&existingBinding); err != nil {
		h.logger.Warn("更新B站凭证到本地文件失败", zap.Error(err))
	}

	h.trackBilibiliBindingSuccess(
		userID,
		fmt.Sprintf("%d", tokenInfo.Mid),
		userName,
		userAvatar,
		userLevel,
		userFans,
		userAttention,
		userCoins,
		userSign,
		userVip,
		existingBinding.IsPrimary,
		"rebind",
	)
	return nil
}

func (h *AccountBindingHandler) trackBilibiliBindingSuccess(appUserID, platformUID, userName, avatar string, level, fans, attention, coins int, sign string, isVIP, isPrimary bool, action string) {
	if h.analytics == nil {
		h.logger.Warn("Analytics客户端未初始化，跳过B站绑定成功事件上报")
		return
	}
	go func() {
		properties := map[string]interface{}{
			"user_id":       platformUID,
			"url":           fmt.Sprintf("https://space.bilibili.com/%s", platformUID),
			"app_user_id":   appUserID,
			"platform_uid":  platformUID,
			"username":      userName,
			"avatar":        avatar,
			"level":         level,
			"fans":          fans,
			"attention":     attention,
			"coins":         coins,
			"sign":          sign,
			"is_vip":        isVIP,
			"is_primary":    isPrimary,
			"action":        action,
			"bind_time":     time.Now().Unix(),
			"resource_type": "account_binding",
		}
		h.analytics.TrackBilibiliAccountBindingSuccess(properties)
		// h.logger.Info("B站绑定成功事件已发送到Analytics客户端",
		// 	zap.String("user_id", userID),
		// 	zap.String("platform_uid", platformUID),
		// 	zap.String("username", userName),
		// 	zap.String("action", action))
	}()
}

// BindingInfo 绑定信息
type BindingInfo struct {
	ID          uint   `json:"id"`
	Platform    string `json:"platform"`
	PlatformUID string `json:"platform_uid"`
	Username    string `json:"username"`
	Avatar      string `json:"avatar"`
	Status      string `json:"status"`
	IsActive    bool   `json:"is_active"`
	IsPrimary   bool   `json:"is_primary"`
	CreateTime  int64  `json:"create_time"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

// GetBindingList godoc
// @Summary 获取账号绑定列表
// @Description 获取指定用户的所有已绑定账号列表
// @Tags account-bindings
// @Accept json
// @Produce json
// @Param user_id query string true "用户ID"
// @Success 200 {object} Response{data=[]BindingInfo}
// @Failure 400 {object} Response "参数错误"
// @Failure 500 {object} Response "服务器错误"
// @Router /api/v1/bindings/list [get]
func (h *AccountBindingHandler) GetBindingList(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		BadRequest(c, "缺少用户ID")
		return
	}

	var bindings []model.AccountBinding
	if err := h.bindingService.GetDB().Where("user_id = ? AND status = ?", userID, model.BindingStatusBound).
		Order("is_primary DESC, created_at DESC").
		Find(&bindings).Error; err != nil {
		h.logger.Error("查询绑定列表失败", zap.Error(err))
		InternalServerError(c, "数据库查询失败")
		return
	}

	bindingInfos := make([]BindingInfo, 0, len(bindings))
	for _, binding := range bindings {
		bindingInfos = append(bindingInfos, BindingInfo{
			ID:          binding.ID,
			Platform:    string(binding.Platform),
			PlatformUID: binding.PlatformUID,
			Username:    binding.Username,
			Avatar:      binding.Avatar,
			Status:      string(binding.Status),
			IsActive:    binding.IsActive(),
			IsPrimary:   binding.IsPrimary,
			CreateTime:  binding.CreatedAt.Unix(),
			LastUsedAt:  binding.LastUsedAt,
			ExpiresAt:   binding.ExpiresAt,
		})
	}

	Success(c, bindingInfos)
}

// SetPrimaryBindingRequest 设置主账号请求
type SetPrimaryBindingRequest struct {
	UserID string `json:"user_id" binding:"required"`
}

// SetPrimaryBinding godoc
// @Summary 设置主账号
// @Description 将指定的 B 站绑定设置为主账号
// @Tags account-bindings
// @Accept json
// @Produce json
// @Param id path int true "绑定ID"
// @Param request body SetPrimaryBindingRequest true "设置主账号请求"
// @Success 200 {object} Response
// @Failure 400 {object} Response "参数错误"
// @Failure 404 {object} Response "绑定记录不存在"
// @Failure 500 {object} Response "服务器错误"
// @Router /api/v1/bindings/{id}/primary [put]
func (h *AccountBindingHandler) SetPrimaryBinding(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		BadRequest(c, "缺少绑定ID")
		return
	}

	var req SetPrimaryBindingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数错误: "+err.Error())
		return
	}

	var binding model.AccountBinding
	if err := h.bindingService.GetDB().Where("id = ? AND user_id = ?", id, req.UserID).First(&binding).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			NotFound(c, "绑定记录不存在")
		} else {
			h.logger.Error("查询绑定记录失败", zap.Error(err))
			InternalServerError(c, "数据库查询失败")
		}
		return
	}

	if binding.Platform != model.PlatformBilibili {
		BadRequest(c, "仅支持设置 B 站主账号")
		return
	}

	var biliData model.BiliPlatformData
	if data, err := binding.GetBiliData(); err != nil {
		h.logger.Error("解析 B 站平台数据失败", zap.Error(err), zap.Uint("binding_id", binding.ID))
		InternalServerError(c, "绑定数据异常")
		return
	} else if data == nil || data.BiliMid == 0 {
		BadRequest(c, "缺少有效的 B 站账号标识")
		return
	} else {
		biliData = *data
	}

	if err := h.biliAccountService.SetPrimaryAccount(req.UserID, biliData.BiliMid); err != nil {
		h.logger.Error("设置 B 站主账号失败", zap.Error(err), zap.String("user_id", req.UserID), zap.Int64("bili_mid", biliData.BiliMid))
		InternalServerError(c, "设置主账号失败")
		return
	}

	Success(c, nil)
}

// UnbindAccount godoc
// @Summary 解绑账号
// @Description 解绑指定的平台账号
// @Tags account-bindings
// @Accept json
// @Produce json
// @Param id path int true "绑定ID"
// @Success 200 {object} Response
// @Failure 400 {object} Response "参数错误"
// @Failure 500 {object} Response "服务器错误"
// @Router /api/v1/bindings/{id} [delete]
func (h *AccountBindingHandler) UnbindAccount(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		BadRequest(c, "缺少绑定ID")
		return
	}

	var binding model.AccountBinding
	if err := h.bindingService.GetDB().First(&binding, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			NotFound(c, "绑定记录不存在")
		} else {
			h.logger.Error("查询绑定记录失败", zap.Error(err))
			InternalServerError(c, "数据库查询失败")
		}
		return
	}

	// 更新状态为已解绑
	binding.Status = model.BindingStatusUnbound
	if err := h.bindingService.GetDB().Save(&binding).Error; err != nil {
		h.logger.Error("更新绑定状态失败", zap.Error(err))
		InternalServerError(c, "数据库更新失败")
		return
	}

	Success(c, nil)
}

// RefreshTokenRequest 刷新令牌请求
type RefreshTokenRequest struct {
	BindingID uint `json:"binding_id" binding:"required"`
}

// RefreshToken godoc
// @Summary 刷新令牌
// @Description 刷新指定绑定的访问令牌
// @Tags account-bindings
// @Accept json
// @Produce json
// @Param request body RefreshTokenRequest true "刷新请求"
// @Success 200 {object} Response
// @Failure 400 {object} Response "参数错误"
// @Failure 500 {object} Response "服务器错误"
// @Router /api/v1/bindings/refresh [post]
func (h *AccountBindingHandler) RefreshToken(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数错误: "+err.Error())
		return
	}

	var binding model.AccountBinding
	if err := h.bindingService.GetDB().First(&binding, req.BindingID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			NotFound(c, "绑定记录不存在")
		} else {
			h.logger.Error("查询绑定记录失败", zap.Error(err))
			InternalServerError(c, "数据库查询失败")
		}
		return
	}

	// TODO: 根据平台实现令牌刷新逻辑
	switch binding.Platform {
	case model.PlatformBilibili:
		// B站令牌刷新逻辑
		h.logger.Warn("B站令牌刷新功能待实现")
		NotImplemented(c, "B站令牌刷新功能待实现")
		return
	default:
		BadRequest(c, "不支持的平台")
		return
	}
}

// buildCookieString 构建正确的cookie字符串
func buildCookieString(cookieInfo map[string]interface{}) string {
	if cookieInfo == nil {
		return ""
	}

	// 检查是否是新的数组格式
	if cookies, ok := cookieInfo["cookies"].([]interface{}); ok {
		cookieParts := []string{}
		for _, cookie := range cookies {
			if cookieMap, ok := cookie.(map[string]interface{}); ok {
				if name, nameOk := cookieMap["name"].(string); nameOk {
					if value, valueOk := cookieMap["value"].(string); valueOk {
						cookieParts = append(cookieParts, fmt.Sprintf("%s=%s", name, value))
					}
				}
			}
		}
		if len(cookieParts) > 0 {
			return strings.Join(cookieParts, "; ")
		}
	}

	// 回退到旧的key-value格式处理
	cookieParts := []string{}
	for key, value := range cookieInfo {
		if key == "cookies" || key == "domains" {
			continue // 跳过特殊字段
		}
		if valueStr, ok := value.(string); ok {
			cookieParts = append(cookieParts, fmt.Sprintf("%s=%s", key, valueStr))
		}
	}

	if len(cookieParts) > 0 {
		return strings.Join(cookieParts, "; ")
	}

	return ""
}

// YouTubeAuthorizeResponse YouTube授权URL响应
type YouTubeAuthorizeResponse struct {
	AuthURL string `json:"auth_url"` // Google OAuth2 授权地址
}

// YouTubeAuthorize godoc
// @Summary 获取YouTube OAuth2授权URL（桌面应用方式）
// @Description 生成YouTube OAuth2授权URL，前端直接跳转完成授权，无需弹窗或二维码
// @Tags account-bindings
// @Accept json
// @Produce json
// @Param user_id query string true "用户ID"
// @Success 200 {object} Response{data=YouTubeAuthorizeResponse}
// @Failure 400 {object} Response "参数错误"
// @Failure 500 {object} Response "服务器错误"
// @Router /api/v1/bindings/youtube/authorize [get]
func (h *AccountBindingHandler) YouTubeAuthorize(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		BadRequest(c, "user_id参数不能为空")
		return
	}

	if h.youtubeClient == nil || h.youtubeClient.OAuthConfig() == nil {
		h.logger.Error("YouTube client未初始化")
		InternalServerError(c, "YouTube OAuth未配置")
		return
	}

	// 检查是否已绑定
	var existingBinding model.AccountBinding
	if err := h.bindingService.GetDB().Where("user_id = ? AND platform = ? AND status = ?",
		userID, model.PlatformYoutube, model.BindingStatusBound).
		First(&existingBinding).Error; err == nil {
		BadRequest(c, "该账号已绑定YouTube平台，请先解绑后再重新绑定")
		return
	}

	// 生成 state key，绑定 user_id
	stateKey := uuid.New().String()

	// 生成 OAuth URL
	authURL := h.youtubeClient.AuthCodeURL(
		stateKey,
		oauth2.AccessTypeOffline,
		oauth2.ApprovalForce,
	)

	// 存储绑定上下文（10分钟过期）
	h.bindingCache.Set(stateKey, map[string]interface{}{
		"user_id":    userID,
		"platform":   "youtube",
		"created_at": time.Now().Unix(),
	}, 10*time.Minute)

	h.logger.Info("生成YouTube OAuth授权URL（桌面应用方式）",
		zap.String("user_id", userID),
		zap.String("state", stateKey),
	)

	Success(c, YouTubeAuthorizeResponse{AuthURL: authURL})
}

// YouTubeCompleteAuthorization godoc
// @Summary 完成 YouTube OAuth bridge 授权导入
// @Description 使用一次性 transfer_token 从线上 bridge 拉取 YouTube 绑定信息并写入本地数据库
// @Tags account-bindings
// @Accept json
// @Produce json
// @Param request body completeYouTubeAuthorizationRequest true "transfer_token"
// @Success 200 {object} Response{data=completeYouTubeAuthorizationResponse}
// @Failure 400 {object} Response "参数错误"
// @Failure 500 {object} Response "服务器错误"
// @Router /api/v1/bindings/youtube/complete [post]
func (h *AccountBindingHandler) YouTubeCompleteAuthorization(c *gin.Context) {
	BadRequest(c, "YouTube OAuth bridge has been removed. Please use the local YouTube OAuth flow instead.")
}

// YouTubeOAuthCallback godoc
// @Summary YouTube OAuth回调
// @Description 处理YouTube OAuth授权回调，完成账号绑定
// @Tags account-bindings
// @Accept json
// @Produce json
// @Param code query string true "授权码"
// @Param state query string true "状态值（qr_code_key）"
// @Success 302 {string} string "重定向到前端"
// @Failure 400 {object} Response "参数错误"
// @Failure 500 {object} Response "服务器错误"
// @Router /api/v1/bindings/youtube/callback [get]
func (h *AccountBindingHandler) YouTubeOAuthCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state") // state是qr_code_key

	if code == "" || state == "" {
		c.Redirect(302, frontendURL(c, "/dashboard/accounts/youtube-callback?error=invalid_parameters"))
		return
	}

	qrCodeKey := state

	// 从缓存获取绑定信息
	var bindingData map[string]interface{}
	if err := h.bindingCache.Get(qrCodeKey, &bindingData); err != nil {
		h.logger.Warn("绑定请求已过期或不存在", zap.String("qr_code_key", qrCodeKey))
		c.Redirect(302, frontendURL(c, "/dashboard/accounts/youtube-callback?error=expired"))
		return
	}

	userID := fmt.Sprintf("%v", bindingData["user_id"])

	// 使用YouTubeHandler的OAuth配置交换token
	if h.youtubeClient == nil || h.youtubeClient.OAuthConfig() == nil {
		h.logger.Error("YouTube client未初始化")
		c.Redirect(302, frontendURL(c, "/dashboard/accounts/youtube-callback?error=oauth_not_configured"))
		return
	}

	ctx, cancel := context.WithTimeout(h.youtubeClient.OAuthContext(), 60*time.Second)
	defer cancel()

	result, err := h.youtubeBinding.CompleteOAuthBinding(ctx, userID, code)
	if err != nil {
		h.logger.Error("交换OAuth token失败", zap.Error(err))
		c.Redirect(302, frontendURL(c, "/dashboard/accounts/youtube-callback?error=token_exchange_failed"))
		return
	}

	// 清理缓存
	h.bindingCache.Delete(qrCodeKey)

	h.logger.Info("YouTube账号绑定成功",
		zap.String("user_id", userID),
		zap.String("channel_id", result.ChannelID),
		zap.String("channel_title", result.ChannelTitle))

	// 异步获取订阅列表
	go func() {
		if err := h.youtubeBinding.SyncSubscriptions(context.Background(), userID, result.Token); err != nil {
			h.logger.Error("保存YouTube订阅列表失败", zap.Error(err), zap.String("user_id", userID))
		}
	}()

	// 重定向到前端成功页面
	c.Redirect(302, frontendURL(c, fmt.Sprintf(
		"/dashboard/accounts/youtube-callback?success=true&platform=youtube&username=%s",
		url.QueryEscape(result.ChannelTitle),
	)))
}

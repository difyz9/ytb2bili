package handler

import (
	"bytes"
	"fmt"
	"github.com/difyz9/bilibili-go-sdk/bilibili"
	"github.com/difyz9/ytb2bili/internal/core"
	"github.com/difyz9/ytb2bili/internal/storage"
	"image/color"
	"image/png"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/skip2/go-qrcode"
)

type AuthHandler struct {
	BaseHandler
}

func NewAuthHandler(app *core.AppServer) *AuthHandler {
	return &AuthHandler{
		BaseHandler: BaseHandler{App: app},
	}
}

// RegisterRoutes 注册认证相关路由
func (h *AuthHandler) RegisterRoutes(server *core.AppServer) {
	api := server.Engine.Group("/api/v1")

	auth := api.Group("/auth")
	{
		auth.GET("/qrcode", h.getQRCode)
		auth.GET("/qrcode/image/:authCode", h.getQRCodeImage)
		auth.POST("/poll", h.pollQRCode)
		auth.GET("/login", h.loadLoginInfo)
		auth.GET("/status", h.checkLoginStatus)
		auth.GET("/userinfo", h.getUserInfo)
		auth.POST("/logout", h.logout)
	}
}

// QRCodeRequest 二维码请求
type QRCodeRequest struct{}

// QRCodeResponse 二维码响应
type QRCodeResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	QRCodeURL string `json:"qr_code_url"`
	AuthCode  string `json:"auth_code"`
}

// getQRCode 获取登录二维码
func (h *AuthHandler) getQRCode(c *gin.Context) {
	client := bilibili.NewClient()

	qrResp, err := client.GetQRCode()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to get QR code: " + err.Error(),
		})
		return
	}

	if qrResp.Code != 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    qrResp.Code,
			"message": "Failed to get QR code",
		})
		return
	}

	// 构造完整的二维码URL
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	host := c.Request.Host
	fullQRCodeURL := fmt.Sprintf("%s://%s/api/v1/auth/qrcode/image/%s", scheme, host, qrResp.Data.AuthCode)

	c.JSON(http.StatusOK, QRCodeResponse{
		Code:      0,
		Message:   "success",
		QRCodeURL: fullQRCodeURL,
		AuthCode:  qrResp.Data.AuthCode,
	})
}

// getQRCodeImage 生成二维码图片
func (h *AuthHandler) getQRCodeImage(c *gin.Context) {
	authCode := c.Param("authCode")
	if authCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Auth code is required",
		})
		return
	}

	// 构造B站二维码URL
	qrURL := fmt.Sprintf("https://passport.bilibili.com/x/passport-tv-login/h5/qrcode/auth?auth_code=%s", authCode)

	// 生成二维码图片
	qrCode, err := qrcode.New(qrURL, qrcode.Medium)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to generate QR code: " + err.Error(),
		})
		return
	}

	// 设置二维码颜色
	qrCode.BackgroundColor = color.RGBA{255, 255, 255, 255} // 白色背景
	qrCode.ForegroundColor = color.RGBA{0, 0, 0, 255}       // 黑色前景

	// 生成PNG图片
	img := qrCode.Image(240)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to encode QR code image: " + err.Error(),
		})
		return
	}

	// 设置响应头
	c.Header("Content-Type", "image/png")
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	// 返回图片数据
	c.Data(http.StatusOK, "image/png", buf.Bytes())
}

// PollQRCodeRequest 轮询二维码请求
type PollQRCodeRequest struct {
	AuthCode string `json:"auth_code" binding:"required"`
}

// PollQRCodeResponse 轮询二维码响应
type PollQRCodeResponse struct {
	Code      int                 `json:"code"`
	Message   string              `json:"message"`
	LoginInfo *bilibili.LoginInfo `json:"login_info,omitempty"`
}

// pollQRCode 轮询二维码登录状态
func (h *AuthHandler) pollQRCode(c *gin.Context) {
	var req PollQRCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request parameters: " + err.Error(),
		})
		return
	}

	fmt.Println("--轮询二维码--")

	client := bilibili.NewClient()

	loginInfo, err := client.PollQRCode(req.AuthCode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Login failed: " + err.Error(),
		})
		return
	}

	// 获取用户完整信息并补充到LoginInfo中
	var userBasicInfo *storage.UserBasicInfo
	if loginInfo.TokenInfo.Mid > 0 {
		// 构建cookie字符串用于API调用
		cookies := buildCookieString(loginInfo.CookieInfo)

		// 优先使用myinfo API获取完整用户信息 (参考biliup-1.1.16)
		myInfo, err := client.GetMyInfoWithRetry(cookies, 2)
		if err == nil {
			// 使用myinfo API的完整信息
			loginInfo.TokenInfo.Uname = myInfo.Uname
			loginInfo.TokenInfo.Face = myInfo.Face
			if myInfo.Mid > 0 {
				loginInfo.TokenInfo.Mid = myInfo.Mid
			}
			// 转换为存储格式
			userBasicInfo = storage.ConvertMyInfoToUserInfo(myInfo)
		} else {
			log.Printf("Warning: Failed to get myinfo: %v", err)
		}
	} // 登录成功后自动保存到本地（包括用户信息）
	store := storage.GetDefaultStore()
	if userBasicInfo != nil {
		// 保存登录信息和用户信息
		if err := store.SaveWithUserInfo(loginInfo, userBasicInfo); err != nil {
			log.Printf("Warning: Failed to save login info with user info: %v", err)
			// 回退到只保存登录信息
			if err := store.Save(loginInfo); err != nil {
				log.Printf("Warning: Failed to save login info: %v", err)
			}
		}
	} else {
		// 只保存登录信息
		if err := store.Save(loginInfo); err != nil {
			log.Printf("Warning: Failed to save login info: %v", err)
		}
	}

	c.JSON(http.StatusOK, PollQRCodeResponse{
		Code:      0,
		Message:   "Login successful",
		LoginInfo: loginInfo,
	})
}

// LoadLoginInfoResponse 加载登录信息响应
type LoadLoginInfoResponse struct {
	Code      int                 `json:"code"`
	Message   string              `json:"message"`
	LoginInfo *bilibili.LoginInfo `json:"login_info,omitempty"`
}

// loadLoginInfo 从本地加载已保存的登录信息
func (h *AuthHandler) loadLoginInfo(c *gin.Context) {
	store := storage.GetDefaultStore()

	loginInfo, err := store.Load()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "No saved login info or login expired: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, LoadLoginInfoResponse{
		Code:      0,
		Message:   "Login info loaded successfully",
		LoginInfo: loginInfo,
	})
}

// CheckLoginStatusResponse 检查登录状态响应
type CheckLoginStatusResponse struct {
	Code       int       `json:"code"`
	Message    string    `json:"message"`
	IsLoggedIn bool      `json:"is_logged_in"`
	User       *UserInfo `json:"user,omitempty"`
}

type UserInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Mid    string `json:"mid"`
	Avatar string `json:"avatar"`
}

// checkLoginStatus 检查本地登录信息是否有效
func (h *AuthHandler) checkLoginStatus(c *gin.Context) {
	store := storage.GetDefaultStore()
	isValid := store.IsValid()

	response := CheckLoginStatusResponse{
		Code:       0,
		Message:    "success",
		IsLoggedIn: isValid,
	}

	// 如果已登录，返回用户信息
	if isValid {
		// 优先从缓存中获取用户信息
		cachedUserInfo, err := store.GetUserInfo()
		if err == nil && cachedUserInfo != nil {
			// 使用缓存的用户信息
			response.User = &UserInfo{
				ID:     fmt.Sprintf("%d", cachedUserInfo.Mid),
				Name:   cachedUserInfo.Name,
				Mid:    fmt.Sprintf("%d", cachedUserInfo.Mid),
				Avatar: cachedUserInfo.Face,
			}
		} else {
			// 没有缓存的用户信息，从API获取
			loginInfo, _ := store.Load()
			if loginInfo != nil {
				client := bilibili.NewClient()

				// 构建cookie字符串
				cookies := buildCookieString(loginInfo.CookieInfo)

				// 尝试使用myinfo API获取完整用户信息 (参考biliup-1.1.16)
				userName := fmt.Sprintf("用户_%d", loginInfo.TokenInfo.Mid) // 默认用户名
				userAvatar := ""
				userMid := fmt.Sprintf("%d", loginInfo.TokenInfo.Mid)

				// 如果登录信息中有用户名，使用它
				if loginInfo.TokenInfo.Uname != "" {
					userName = loginInfo.TokenInfo.Uname
				}
				if loginInfo.TokenInfo.Face != "" {
					userAvatar = loginInfo.TokenInfo.Face
				}

				var userBasicInfo *storage.UserBasicInfo

				// 优先使用myinfo API获取最新用户信息
				myInfo, err := client.GetMyInfoWithRetry(cookies, 2)
				if err == nil {
					// 使用myinfo API的完整信息
					userName = myInfo.Uname
					userAvatar = myInfo.Face
					userMid = fmt.Sprintf("%d", myInfo.Mid)

					// 更新并保存登录信息和用户信息
					loginInfo.TokenInfo.Uname = myInfo.Uname
					loginInfo.TokenInfo.Face = myInfo.Face
					if myInfo.Mid > 0 {
						loginInfo.TokenInfo.Mid = myInfo.Mid
					}
					userBasicInfo = storage.ConvertMyInfoToUserInfo(myInfo)
				} else {
					log.Printf("Warning: Failed to get myinfo: %v", err)
				} // 保存更新后的信息（包括用户信息）
				if userBasicInfo != nil {
					store.SaveWithUserInfo(loginInfo, userBasicInfo)
				} else {
					store.Save(loginInfo)
				}

				response.User = &UserInfo{
					ID:     userMid,
					Name:   userName,
					Mid:    userMid,
					Avatar: userAvatar,
				}
			}
		}
	}

	c.JSON(http.StatusOK, response)
}

// GetUserInfoResponse 获取用户信息响应
type GetUserInfoResponse struct {
	Code     int                     `json:"code"`
	Message  string                  `json:"message"`
	UserInfo *bilibili.UserBasicInfo `json:"user_info,omitempty"`
}

// getUserInfo 获取当前登录用户的详细信息
func (h *AuthHandler) getUserInfo(c *gin.Context) {
	store := storage.GetDefaultStore()
	if !store.IsValid() {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "User not logged in",
		})
		return
	}

	loginInfo, err := store.Load()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to load login info: " + err.Error(),
		})
		return
	}

	client := bilibili.NewClient()

	// 构建cookie字符串
	cookies := buildCookieString(loginInfo.CookieInfo)

	// 优先使用myinfo API获取用户信息 (参考biliup-1.1.16)
	myInfo, err := client.GetMyInfoWithRetry(cookies, 3)
	if err != nil {
		log.Printf("Failed to get myinfo: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to get user info: " + err.Error(),
		})
		return
	}

	// 使用myinfo API返回的完整信息
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    myInfo,
	})
}

// LogoutResponse 登出响应
type LogoutResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// logout 删除本地保存的登录信息（登出）
func (h *AuthHandler) logout(c *gin.Context) {
	store := storage.GetDefaultStore()

	if err := store.Delete(); err != nil {
		log.Printf("Warning: Failed to delete login info: %v", err)
	}

	c.JSON(http.StatusOK, LogoutResponse{
		Code:    0,
		Message: "Logout successful",
	})
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

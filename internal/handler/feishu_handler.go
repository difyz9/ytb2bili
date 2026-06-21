// Package handler — Feishu (Lark) bot webhook handler.
//
// 设计思路（借鉴 nanobot-go Channel 接口）：
//   - 飞书通过 HTTP 事件回调（POST /webhook/feishu）推送消息
//   - 收到 im.message.receive_v1 事件后，解析 YouTube URL
//   - 立即 Reply "⏳ 处理中..."，然后异步触发 YouTubeChain 工作流
//   - 处理完成后通过飞书 API 发送结果通知
//
// 前置配置（config.toml）：
//
//	[feishu]
//	enabled            = true
//	app_id             = "cli_xxx"
//	app_secret         = "xxxx"
//	verification_token = "xxxx"   # 飞书控制台"事件订阅 > 验证令牌"
//	encrypt_key        = ""       # 留空则不加密
package handler

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zolagz/ytb2bili/internal/config"
	"github.com/zolagz/ytb2bili/internal/service"
	"github.com/zolagz/ytb2bili/internal/workflow"
	"github.com/zolagz/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const feishuBaseURL = "https://open.feishu.cn/open-apis"

// ─────────────────────────────────────────────────────────────────────────────
// FeishuHandler
// ─────────────────────────────────────────────────────────────────────────────

// FeishuHandler handles Feishu bot webhook events.
// When disabled in config, all methods are no-ops.
type FeishuHandler struct {
	cfg          *config.FeishuConfig
	youtubeChain *workflow.YouTubeChain
	userSettings *service.UserSettingsClient
	videoService *service.VideoService
	logger       *zap.Logger

	// tenant_access_token cache (thread-safe)
	mu          sync.Mutex
	accessToken string
	tokenExpiry time.Time
}

// FeishuHandlerParams groups FeishuHandler's fx dependencies.
type FeishuHandlerParams struct {
	AppCfg       *config.AppConfig
	Chain        *workflow.YouTubeChain
	UserSettings *service.UserSettingsClient
	VideoService *service.VideoService
	Logger       *zap.Logger
}

// NewFeishuHandler creates a FeishuHandler.
func NewFeishuHandler(appCfg *config.AppConfig, chain *workflow.YouTubeChain, userSettings *service.UserSettingsClient, videoService *service.VideoService, logger *zap.Logger) *FeishuHandler {
	return &FeishuHandler{
		cfg:          &appCfg.Feishu,
		youtubeChain: chain,
		userSettings: userSettings,
		videoService: videoService,
		logger:       logger.With(zap.String("handler", "feishu")),
	}
}

// RegisterRoutes registers POST /webhook/feishu on the Gin engine.
// The route is only registered when feishu.enabled = true.
func (h *FeishuHandler) RegisterRoutes(r *gin.Engine) {
	if !h.cfg.Enabled {
		h.logger.Info("⏭️  飞书 Bot 未启用，跳过路由注册")
		return
	}
	r.POST("/webhook/feishu", h.Webhook)
	h.logger.Info("✅ 飞书 Bot 路由已注册", zap.String("path", "POST /webhook/feishu"))
}

// ─────────────────────────────────────────────────────────────────────────────
// Webhook — main entry-point
// ─────────────────────────────────────────────────────────────────────────────

// Webhook handles all inbound Feishu event callbacks.
//
// Supported event flows:
//  1. URL verification (type = "url_verification") → echo challenge
//  2. im.message.receive_v1 → trigger YouTube processing pipeline
func (h *FeishuHandler) Webhook(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.logger.Error("读取请求体失败", zap.Error(err))
		c.Status(http.StatusBadRequest)
		return
	}

	// ── 1. Decrypt if encrypt_key is configured ────────────────────────────
	payload := body
	if h.cfg.EncryptKey != "" {
		var enc struct {
			Encrypt string `json:"encrypt"`
		}
		if err := json.Unmarshal(body, &enc); err == nil && enc.Encrypt != "" {
			decrypted, err := feishuDecrypt(h.cfg.EncryptKey, enc.Encrypt)
			if err != nil {
				h.logger.Error("飞书事件解密失败", zap.Error(err))
				c.Status(http.StatusBadRequest)
				return
			}
			payload = []byte(decrypted)
		}
	}

	// ── 2. URL verification challenge ─────────────────────────────────────
	var challenge struct {
		Challenge string `json:"challenge"`
		Token     string `json:"token"`
		Type      string `json:"type"`
	}
	if err := json.Unmarshal(payload, &challenge); err == nil &&
		challenge.Type == "url_verification" {
		h.logger.Info("飞书 URL 校验请求", zap.String("challenge", challenge.Challenge))
		c.JSON(http.StatusOK, gin.H{"challenge": challenge.Challenge})
		return
	}

	// ── 3. Parse event ────────────────────────────────────────────────────
	var event feishuEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		h.logger.Error("解析飞书事件失败", zap.Error(err))
		c.Status(http.StatusBadRequest)
		return
	}

	// ── 4. Verify token ───────────────────────────────────────────────────
	if h.cfg.VerificationToken != "" && event.Header.Token != h.cfg.VerificationToken {
		h.logger.Warn("飞书事件 token 校验失败",
			zap.String("received", event.Header.Token))
		c.Status(http.StatusForbidden)
		return
	}

	// Acknowledge immediately; processing is asynchronous.
	c.Status(http.StatusOK)

	// ── 5. Route event ────────────────────────────────────────────────────
	switch event.Header.EventType {
	case "im.message.receive_v1":
		go h.handleIMMessage(event)
	default:
		// Ignore other event types silently.
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// IM message handler
// ─────────────────────────────────────────────────────────────────────────────

func (h *FeishuHandler) handleIMMessage(event feishuEvent) {
	msg := event.Event.Message
	if msg.MessageType != "text" {
		return
	}

	// Parse text content from JSON: {"text": "..."}
	var textBody struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(msg.Content), &textBody); err != nil {
		h.logger.Warn("解析飞书消息内容失败", zap.Error(err))
		return
	}

	text := strings.TrimSpace(textBody.Text)
	openID := event.Event.Sender.SenderID.OpenID

	// Only react to messages that contain a YouTube URL.
	ytURL := extractYouTubeURL(text)
	if ytURL == "" {
		h.logger.Debug("飞书消息不含 YouTube URL，忽略", zap.String("text", text))
		return
	}

	videoID := extractYouTubeVideoID(ytURL)
	h.logger.Info("收到 YouTube 处理请求",
		zap.String("open_id", openID),
		zap.String("url", ytURL),
		zap.String("video_id", videoID))

	// Reply immediately so the user gets instant feedback.
	replyText := fmt.Sprintf("⏳ 已收到视频链接，正在处理中...\n🔗 %s", ytURL)
	if err := h.replyMessage(msg.MessageID, replyText); err != nil {
		h.logger.Warn("发送飞书确认消息失败", zap.Error(err))
	}

	// Pre-create DB record so the progress tracker has a row to update.
	if videoID != "" {
		var existing model.Video
		if h.videoService.GetDB().Where("video_id = ?", videoID).First(&existing).Error == gorm.ErrRecordNotFound {
			newVideo := &model.Video{
				VideoID:       videoID,
				URL:           ytURL,
				UserID:        openID,
				Platform:      "youtube",
				Status:        model.VideoStatusProcessing,
				OperationType: "feishu",
			}
			if err := h.videoService.GetDB().Create(newVideo).Error; err != nil {
				h.logger.Warn("创建视频记录失败", zap.Error(err))
			}
		} else {
			h.videoService.GetDB().Model(&existing).Updates(map[string]interface{}{
				"status":         model.VideoStatusProcessing,
				"operation_type": "feishu",
			})
		}
	}

	// Run the full YouTube workflow pipeline (may take several minutes).
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	ctx = workflow.WithUserID(ctx, openID)
	preferences := ResolveVideoProcessingPreferences(ctx, h.userSettings, openID, "", nil, nil)

	if videoID != "" {
		if err := h.videoService.GetDB().Model(&model.Video{}).
			Where("video_id = ?", videoID).
			Updates(map[string]interface{}{
				"preferred_resolution": preferences.PreferredResolution,
				"speech_voice_name":    preferences.SpeechSynthesisConfig.VoiceName,
				"task_chain_settings":  serializeTaskChainSettings(preferences.TaskChainSettings),
			}).Error; err != nil {
			h.logger.Warn("更新飞书视频偏好失败", zap.String("video_id", videoID), zap.Error(err))
		}
	}

	var result *workflow.VideoContext
	var processErr error
	if videoID != "" {
		initialCtx := &workflow.VideoContext{
			VideoURL:              ytURL,
			VideoID:               videoID,
			UserID:                openID,
			PreferredResolution:   preferences.PreferredResolution,
			SpeechSynthesisConfig: preferences.SpeechSynthesisConfig,
			TaskChainSettings:     preferences.TaskChainSettings,
		}
		result, processErr = h.youtubeChain.ProcessContextWithTracking(ctx, initialCtx, videoID, openID)
	} else {
		result, processErr = h.youtubeChain.Process(ctx, ytURL)
	}

	if processErr != nil {
		h.logger.Error("飞书视频处理失败",
			zap.String("url", ytURL),
			zap.Error(processErr))
		if videoID != "" {
			h.videoService.GetDB().Model(&model.Video{}).
				Where("video_id = ?", videoID).
				Update("status", model.VideoStatusFailed)
		}
		errMsg := fmt.Sprintf("❌ 视频处理失败\n🔗 %s\n\n原因：%v", ytURL, processErr)
		if err := h.sendMessage(openID, errMsg); err != nil {
			h.logger.Warn("发送飞书错误通知失败", zap.Error(err))
		}
		return
	}

	// Build success reply.
	successMsg := buildSuccessMessage(ytURL, result)
	if err := h.sendMessage(openID, successMsg); err != nil {
		h.logger.Warn("发送飞书完成通知失败", zap.Error(err))
	}
}

func buildSuccessMessage(ytURL string, result *workflow.VideoContext) string {
	var sb strings.Builder
	sb.WriteString("✅ 视频处理完成！\n")
	sb.WriteString(fmt.Sprintf("🔗 %s\n", ytURL))

	if result != nil {
		if result.Title != "" {
			sb.WriteString(fmt.Sprintf("📌 标题：%s\n", result.Title))
		}
		if result.BiliBVID != "" {
			sb.WriteString(fmt.Sprintf("🎬 B站视频：https://www.bilibili.com/video/%s\n", result.BiliBVID))
		} else if result.VideoPath != "" {
			sb.WriteString("📁 视频已下载到本地\n")
		}
		if result.Transcript != nil && result.Transcript.SRTPath != "" {
			sb.WriteString("📝 字幕生成完成\n")
		}
	}
	return sb.String()
}

// ─────────────────────────────────────────────────────────────────────────────
// Feishu API helpers
// ─────────────────────────────────────────────────────────────────────────────

// getTenantAccessToken returns a cached or freshly-fetched tenant_access_token.
func (h *FeishuHandler) getTenantAccessToken() (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Return cached token if still valid (with 60s safety margin).
	if h.accessToken != "" && time.Now().Add(60*time.Second).Before(h.tokenExpiry) {
		return h.accessToken, nil
	}

	body, _ := json.Marshal(map[string]string{
		"app_id":     h.cfg.AppID,
		"app_secret": h.cfg.AppSecret,
	})

	resp, err := http.Post(
		feishuBaseURL+"/auth/v3/tenant_access_token/internal/",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return "", fmt.Errorf("获取飞书 token 失败: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析飞书 token 响应失败: %w", err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("飞书 token API 错误 (code=%d): %s", result.Code, result.Msg)
	}

	h.accessToken = result.TenantAccessToken
	h.tokenExpiry = time.Now().Add(time.Duration(result.Expire) * time.Second)
	h.logger.Debug("飞书 token 已刷新", zap.Int("expire_secs", result.Expire))
	return h.accessToken, nil
}

// replyMessage sends a text reply to a specific message (thread reply).
func (h *FeishuHandler) replyMessage(messageID, text string) error {
	token, err := h.getTenantAccessToken()
	if err != nil {
		return err
	}

	content, _ := json.Marshal(map[string]string{"text": text})
	body, _ := json.Marshal(map[string]string{
		"msg_type": "text",
		"content":  string(content),
	})

	url := fmt.Sprintf("%s/im/v1/messages/%s/reply", feishuBaseURL, messageID)
	return h.doPost(url, token, body)
}

// sendMessage sends a new message to an open_id (non-reply, e.g., for async completion).
func (h *FeishuHandler) sendMessage(openID, text string) error {
	token, err := h.getTenantAccessToken()
	if err != nil {
		return err
	}

	content, _ := json.Marshal(map[string]string{"text": text})
	body, _ := json.Marshal(map[string]interface{}{
		"receive_id": openID,
		"msg_type":   "text",
		"content":    string(content),
	})

	url := feishuBaseURL + "/im/v1/messages?receive_id_type=open_id"
	return h.doPost(url, token, body)
}

func (h *FeishuHandler) doPost(url, token string, body []byte) error {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("飞书 API 请求失败: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("解析飞书 API 响应失败: %w", err)
	}
	if result.Code != 0 {
		return fmt.Errorf("飞书 API 错误 (code=%d): %s", result.Code, result.Msg)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Event payload types
// ─────────────────────────────────────────────────────────────────────────────

// feishuEvent represents the Feishu v2.0 event envelope.
type feishuEvent struct {
	Schema string `json:"schema"`
	Header struct {
		EventID   string `json:"event_id"`
		EventType string `json:"event_type"`
		Token     string `json:"token"`
		AppID     string `json:"app_id"`
	} `json:"header"`
	Event struct {
		Sender struct {
			SenderID struct {
				UnionID string `json:"union_id"`
				UserID  string `json:"user_id"`
				OpenID  string `json:"open_id"`
			} `json:"sender_id"`
			SenderType string `json:"sender_type"`
		} `json:"sender"`
		Message struct {
			MessageID   string `json:"message_id"`
			CreateTime  string `json:"create_time"`
			ChatID      string `json:"chat_id"`
			ChatType    string `json:"chat_type"`
			MessageType string `json:"message_type"`
			Content     string `json:"content"`
		} `json:"message"`
	} `json:"event"`
}

// ─────────────────────────────────────────────────────────────────────────────
// AES-256-CBC decryption
// ─────────────────────────────────────────────────────────────────────────────

// feishuDecrypt decrypts a Feishu AES-CBC-encrypted event payload.
// Algorithm: Key = SHA256(encryptKey); IV = first 16 bytes of decoded ciphertext.
func feishuDecrypt(encryptKey, encryptedStr string) (string, error) {
	// Key = SHA-256 of the encrypt key string
	hash := sha256.Sum256([]byte(encryptKey))
	key := hash[:]

	ciphertext, err := base64.StdEncoding.DecodeString(encryptedStr)
	if err != nil {
		return "", fmt.Errorf("base64 解码失败: %w", err)
	}
	if len(ciphertext) < aes.BlockSize {
		return "", fmt.Errorf("密文长度不足")
	}

	iv := ciphertext[:aes.BlockSize]
	data := ciphertext[aes.BlockSize:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("创建 AES cipher 失败: %w", err)
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(data, data)

	// PKCS7 unpad
	if len(data) == 0 {
		return "", fmt.Errorf("解密后数据为空")
	}
	padLen := int(data[len(data)-1])
	if padLen == 0 || padLen > aes.BlockSize {
		return "", fmt.Errorf("PKCS7 padding 无效")
	}
	return string(data[:len(data)-padLen]), nil
}

// ─────────────────────────────────────────────────────────────────────────────
// URL helpers
// ─────────────────────────────────────────────────────────────────────────────

var youtubeURLRegex = regexp.MustCompile(
	`https?://(?:www\.)?(?:youtube\.com/(?:watch\?v=|shorts/|embed/)|youtu\.be/)[\w\-]{11}[^\s]*`,
)

// extractYouTubeURL returns the first YouTube URL found in s, or "".
func extractYouTubeURL(s string) string {
	match := youtubeURLRegex.FindString(s)
	return match
}

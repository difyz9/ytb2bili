package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zolagz/ytb2bili/internal/config"
	"github.com/zolagz/ytb2bili/internal/middleware"
	"github.com/zolagz/ytb2bili/internal/service"
	"github.com/zolagz/ytb2bili/pkg/tools"
	"go.uber.org/zap"
)

type TTSHandler struct {
	catalog   service.TTSVoiceCatalog
	client    *tools.TTSClient
	logger    *zap.Logger
	jwtSecret string
}

func NewTTSHandler(catalog service.TTSVoiceCatalog, client *tools.TTSClient, logger *zap.Logger, cfg *config.AppConfig) *TTSHandler {
	jwtSecret := ""
	if cfg != nil {
		jwtSecret = strings.TrimSpace(cfg.Auth.JWTSecret)
	}

	return &TTSHandler{
		catalog:   catalog,
		client:    client,
		logger:    logger,
		jwtSecret: jwtSecret,
	}
}

type PreviewTTSRequest struct {
	Text      string  `json:"text"`
	Provider  string  `json:"provider"`
	VoiceName string  `json:"voice_name"`
	Language  string  `json:"language"`
	Format    string  `json:"format"`
	Rate      float64 `json:"rate"`
	Volume    float64 `json:"volume"`
	Pitch     float64 `json:"pitch"`
}

const defaultPreviewTTSContent = "你好，很高兴为您服务！这是一个语音合成的示例。"

type GetTTSVoicesResponse struct {
	Total   int                      `json:"total"`
	Voices  []service.TTSVoiceRecord `json:"voices"`
	Cascade *service.TTSVoiceCascade `json:"cascade"`
}

func (h *TTSHandler) GetVoices(c *gin.Context) {
	voices, err := h.catalog.List(c.Request.Context())
	if err != nil {
		InternalServerError(c, ErrInternalServer)
		return
	}

	cascade, err := h.catalog.Cascade(c.Request.Context())
	if err != nil {
		InternalServerError(c, ErrInternalServer)
		return
	}

	Success(c, GetTTSVoicesResponse{
		Total:   len(voices),
		Voices:  voices,
		Cascade: cascade,
	})
}

func (h *TTSHandler) PreviewVoice(c *gin.Context) {
	uid := strings.TrimSpace(c.GetString("uid"))
	if uid == "" {
		Unauthorized(c, "未授权，请先登录")
		return
	}
	if h.client == nil {
		ServiceUnavailable(c, "TTS 试听服务未配置")
		return
	}

	var req PreviewTTSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "请求参数错误")
		return
	}

	voiceName := strings.TrimSpace(req.VoiceName)
	if voiceName == "" {
		BadRequest(c, "voice_name 不能为空")
		return
	}

	text := strings.TrimSpace(req.Text)
	if text == "" {
		text = defaultPreviewTTSContent
	}

	result, err := h.client.SynthesizeSpeech(c.Request.Context(), tools.TTSRequest{
		UserID:    uid,
		Text:      text,
		Provider:  strings.TrimSpace(req.Provider),
		VoiceName: voiceName,
		Language:  strings.TrimSpace(req.Language),
		Format:    strings.TrimSpace(req.Format),
		Rate:      req.Rate,
		Volume:    req.Volume,
		Pitch:     req.Pitch,
	})
	if err != nil {
		if h.logger != nil {
			h.logger.Warn("TTS 试听失败",
				zap.String("uid", uid),
				zap.String("voice_name", voiceName),
				zap.Error(err))
		}
		BadRequest(c, err.Error())
		return
	}

	if len(result.Audio) == 0 {
		InternalServerError(c, "试听音频生成失败")
		return
	}

	contentType := "audio/mpeg"
	if strings.Contains(strings.ToLower(result.Format), "wav") {
		contentType = "audio/wav"
	}

	c.Header("Cache-Control", "no-store")
	c.Header("Content-Disposition", "inline; filename=voice-preview")
	c.Data(http.StatusOK, contentType, result.Audio)
}

func (h *TTSHandler) RegisterRoutes(r *gin.Engine) {
	g := r.Group("/api/v1/tts")
	g.GET("/voices", h.GetVoices)

	secured := g.Group("")
	secured.Use(middleware.AnyAuthMiddleware(h.jwtSecret))
	secured.POST("/preview", h.PreviewVoice)
}

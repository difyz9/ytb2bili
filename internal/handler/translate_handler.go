package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zolagz/ytb2bili/internal/config"
	"github.com/zolagz/ytb2bili/internal/service"
	"github.com/zolagz/ytb2bili/pkg/llm"
	"github.com/zolagz/ytb2bili/pkg/tools"
	"go.uber.org/zap"
)

type TranslateHandler struct {
	userSettings *service.UserSettingsClient
	cfg          *config.AppConfig
	logger       *zap.Logger
}

func NewTranslateHandler(userSettings *service.UserSettingsClient, cfg *config.AppConfig, logger *zap.Logger) *TranslateHandler {
	return &TranslateHandler{userSettings: userSettings, cfg: cfg, logger: logger}
}

// RegisterRoutes 注册翻译路由
func (h *TranslateHandler) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api/v1/translate")
	{
		api.POST("/subtitles", h.TranslateSubtitles)
	}
}

type SubtitleFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type TranslateRequest struct {
	Files []SubtitleFile `json:"files"`
	From  string         `json:"from"`
	To    string         `json:"to"`
}

type TranslateResponse struct {
	Files []SubtitleFile `json:"files"`
}

// TranslateSubtitles 翻译字幕（保留时间轴）。请求示例：{files:[{path,content}],from:"auto",to:"zh"}
func (h *TranslateHandler) TranslateSubtitles(c *gin.Context) {
	var req TranslateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.From == "" {
		req.From = "auto"
	}
	if req.To == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target language 'to' is required"})
		return
	}

	llmClient, translationModel, err := h.createLLMClient(c)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}

	outFiles := make([]SubtitleFile, 0, len(req.Files))
	for _, f := range req.Files {
		translated, err := h.translateSRT(c, llmClient, translationModel, f.Content, req.From, req.To)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("translate %s failed: %v", f.Path, err)})
			return
		}
		outFiles = append(outFiles, SubtitleFile{Path: f.Path, Content: translated})
	}

	c.JSON(http.StatusOK, TranslateResponse{Files: outFiles})
}

// createLLMClient creates an LLM client using the user's configured LLM settings,
// falling back to the global config.
func (h *TranslateHandler) createLLMClient(c *gin.Context) (*llm.EinoChatClient, string, error) {
	userID := strings.TrimSpace(c.GetString("uid"))

	// Get base provider config
	p := h.cfg.ResolveChatProvider()
	if p == nil || strings.TrimSpace(p.APIKey) == "" {
		return nil, "", fmt.Errorf("LLM API key not configured. Set [chat] or [llm] api_key in config.toml")
	}

	modelName := h.resolveTranslationModel(c)

	// Allow user-level override of base URL and API key
	baseURL := p.BaseURL
	apiKey := p.APIKey
	if userID != "" && h.userSettings != nil && h.userSettings.IsEnabled() {
		settings, err := h.userSettings.GetSettings(c.Request.Context(), userID)
		if err == nil {
			if v := strings.TrimSpace(settings["llm_base_url"]); v != "" {
				baseURL = strings.TrimRight(v, "/")
			}
			if v := strings.TrimSpace(settings["llm_api_key"]); v != "" {
				apiKey = v
			}
		}
	}

	client, err := llm.NewClientFromConfig(&llm.ProviderConfig{
		Provider: p.Provider,
		Model:    modelName,
		BaseURL:  baseURL,
		APIKey:   apiKey,
		Temperature: p.Temperature,
		MaxTokens:   p.MaxTokens,
		Timeout:     p.Timeout,
	}, h.logger)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create LLM client: %w", err)
	}

	return client, modelName, nil
}

func (h *TranslateHandler) resolveTranslationModel(c *gin.Context) string {
	// Check chat/llm config first
	p := h.cfg.ResolveChatProvider()
	if p != nil && p.Model != "" {
		return p.Model
	}

	// Fallback to default
	modelName := llm.DefaultTranslationModel
	userID := strings.TrimSpace(c.GetString("uid"))
	if userID == "" || h.userSettings == nil || !h.userSettings.IsEnabled() {
		return modelName
	}

	settings, err := h.userSettings.GetSettings(c.Request.Context(), userID)
	if err != nil {
		return modelName
	}
	if configured := strings.TrimSpace(settings["translation_model"]); configured != "" {
		return configured
	}
	if configured := strings.TrimSpace(settings["llm_model"]); configured != "" {
		return configured
	}
	return modelName
}

func (h *TranslateHandler) translateSRT(c *gin.Context, llmClient *llm.EinoChatClient, modelName, content, from, to string) (string, error) {
	entries, err := tools.ParseSRTContent(content)
	if err != nil {
		return "", fmt.Errorf("parse SRT failed: %w", err)
	}
	if len(entries) == 0 {
		return content, nil
	}

	texts := make([]string, 0, len(entries))
	for _, entry := range entries {
		texts = append(texts, entry.Text)
	}

	translator := tools.NewBatchTranslator(llmClient, tools.BatchTranslatorConfig{
		SourceLang:  from,
		TargetLang:  to,
		BatchSize:   h.cfg.Workflow.LLMTranslationBatchSize,
		MaxWorkers:  h.cfg.Workflow.LLMTranslationMaxWorkers,
		ContextSize: h.cfg.Workflow.LLMTranslationContextSize,
	}, h.logger)

	result, err := translator.TranslateTextsWithConfig(c.Request.Context(), texts, tools.TranslationRunConfig{
		SourceLang: from,
		TargetLang: to,
		ModelName:  modelName,
		UserID:     strings.TrimSpace(c.GetString("uid")),
	})
	if err != nil {
		return "", err
	}

	return tools.GenerateSRTContent(entries, result.TranslatedTexts), nil
}

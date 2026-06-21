package handler

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zolagz/ytb2bili/internal/config"
	"github.com/zolagz/ytb2bili/internal/service"
	agent "github.com/zolagz/ytb2bili/pkg/agent"
	"github.com/zolagz/ytb2bili/pkg/llm"
	"github.com/zolagz/ytb2bili/pkg/store/model"
	storemodel "github.com/zolagz/ytb2bili/pkg/store/model"
	"github.com/zolagz/ytb2bili/pkg/tools"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// AgentHandler handles AI agent HTTP requests.
type AgentHandler struct {
	agent        *agent.NanoAgent
	cfg          *config.AppConfig
	db           *gorm.DB
	userSettings *service.UserSettingsClient
	logger       *zap.Logger
}

type AgentModelOption struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	MinTier     string `json:"min_tier"`
}

var agentModelMetadata = map[string]AgentModelOption{
	"gpt-4o-mini":      {ID: "gpt-4o-mini", Label: "GPT-4o Mini", Description: "适合日常问答和轻量任务", MinTier: string(model.TierFree)},
	"gpt-4o":           {ID: "gpt-4o", Label: "GPT-4o", Description: "更强的综合能力，适合高质量内容生成", MinTier: string(model.TierPro)},
	"deepseek-chat":    {ID: "deepseek-chat", Label: "DeepSeek Chat", Description: "推理性更强，适合复杂指令", MinTier: string(model.TierBasic)},
	"deepseek-reasoner": {ID: "deepseek-reasoner", Label: "DeepSeek Reasoner", Description: "更强推理能力，适合复杂分析", MinTier: string(model.TierPro)},
	"gemini-2.0-flash": {ID: "gemini-2.0-flash", Label: "Gemini 2.0 Flash", Description: "更快的多轮对话和长文本处理", MinTier: string(model.TierEnterprise)},
}

// NewAgentHandler creates a new AgentHandler.
func NewAgentHandler(a *agent.NanoAgent, cfg *config.AppConfig, logger *zap.Logger, db *gorm.DB, userSettings *service.UserSettingsClient) *AgentHandler {
	return &AgentHandler{
		agent:        a,
		cfg:          cfg,
		db:           db,
		userSettings: userSettings,
		logger:       logger,
	}
}

// agentAvailable returns true when the underlying NanoAgent is ready.
func (h *AgentHandler) agentAvailable() bool {
	return h.agent != nil
}

// Info godoc
// @Summary      Get agent information
// @Description  Get agent name, available tools, and their parameter schemas
// @Tags         agent
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /agent/info [get]
func (h *AgentHandler) Info(c *gin.Context) {
	if !h.agentAvailable() {
		c.JSON(http.StatusOK, gin.H{
			"available": false,
			"message":   "AI 助手当前不可用",
		})
		return
	}

	userID, role := h.getUserIdentity(c)
	tier := h.resolveTier(userID, role)
	availableModels := h.allowedModels(c.Request.Context(), userID, role, tier)

	toolInfos := make([]map[string]string, 0, len(h.agent.Tools))
	for _, t := range h.agent.Tools {
		info, err := t.Info(c.Request.Context())
		if err != nil || info == nil {
			continue
		}
		toolInfos = append(toolInfos, map[string]string{
			"name": info.Name,
			"desc": info.Desc,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"available":        true,
		"name":             h.agent.Name,
		"tools":            toolInfos,
		"available_models": availableModels,
		"membership_tier":  tier,
	})
}

// RunRequest is the request body for the Run endpoint.
type RunRequest struct {
	Query string `json:"query" binding:"required"`
	Model string `json:"model,omitempty"`
}

// Run godoc
// @Summary      Execute natural language task
// @Description  Submit a natural language task; AI agent orchestrates tools automatically via native function calling
// @Tags         agent
// @Accept       json
// @Produce      json
// @Param        request  body      RunRequest  true  "Task description"
// @Success      200      {object}  map[string]interface{}
// @Failure      400      {object}  map[string]interface{}
// @Failure      500      {object}  map[string]interface{}
// @Router       /agent/run [post]
func (h *AgentHandler) Run(c *gin.Context) {
	if !h.agentAvailable() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "agent_not_configured",
			"message": "AI 助手当前不可用",
		})
		return
	}

	var req RunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	userID, role := h.getUserIdentity(c)
	tier := h.resolveTier(userID, role)

	if req.Model != "" && !h.canUseModel(c.Request.Context(), userID, req.Model, role, tier) {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "model_forbidden",
			"message": "当前会员等级无法使用该聊天模型，请升级会员后重试",
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Minute)
	defer cancel()

	tools.InjectUserContext(h.agent.Tools, userID)

	selectedModel := strings.TrimSpace(req.Model)
	if selectedModel == "" {
		selectedModel = h.resolvePreferredModel(ctx, userID, role, tier)
	}

	var result *agent.RunResult
	var err error
	if selectedModel != "" {
		result, err = h.agent.RunWithModel(ctx, req.Query, selectedModel)
	} else {
		result, err = h.agent.Run(ctx, req.Query)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "agent_error", "message": err.Error()})
		return
	}

	steps := make([]map[string]interface{}, 0, len(result.Steps))
	for _, s := range result.Steps {
		entry := map[string]interface{}{
			"tool":      s.Tool,
			"arguments": s.Arguments,
			"output":    s.Output,
		}
		if s.Error != "" {
			entry["error"] = s.Error
		}
		steps = append(steps, entry)
	}

	c.JSON(http.StatusOK, gin.H{
		"result":       result.FinalAnswer,
		"success":      result.Success,
		"steps":        steps,
		"execution_ms": result.Duration.Milliseconds(),
		"model":        selectedModel,
	})
}

func (h *AgentHandler) resolvePreferredModel(ctx context.Context, userID, role string, tier model.Tier) string {
	if userID != "" && h.userSettings != nil && h.userSettings.IsEnabled() {
		settings, err := h.userSettings.GetSettings(ctx, userID)
		if err != nil {
			h.logger.Warn("load agent preferred model failed", zap.String("user_id", userID), zap.Error(err))
		} else if preferred := strings.TrimSpace(settings[storemodel.UserSettingKeyPreferredAIModel]); preferred != "" {
			if h.canUseModel(ctx, userID, preferred, role, tier) {
				return preferred
			}
		}
	}

	return ""
}

// RegisterRoutes registers agent API routes.
func (h *AgentHandler) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api")
	api.GET("/agent/info", h.Info)
	api.POST("/agent/run", h.Run)
}

func (h *AgentHandler) getUserIdentity(c *gin.Context) (string, string) {
	userID := c.GetString("uid")
	role := c.GetString("role")
	return userID, role
}

func (h *AgentHandler) resolveTier(userID, role string) model.Tier {
	if role == "admin" {
		return model.TierEnterprise
	}
	if userID == "" || h.db == nil {
		return model.TierFree
	}

	var membership model.UserMembership
	if err := h.db.Where("user_id = ?", userID).First(&membership).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			h.logger.Warn("query membership for agent access failed", zap.String("user_id", userID), zap.Error(err))
		}
		return model.TierFree
	}

	if membership.ExpiresAt.After(time.Now()) {
		return membership.Tier
	}

	return model.TierFree
}

func (h *AgentHandler) allowedModels(ctx context.Context, userID, role string, tier model.Tier) []AgentModelOption {
	baseCatalog := h.providerModelCatalog()

	if role == "admin" {
		return h.mergeDynamicModels(ctx, userID, append([]AgentModelOption(nil), baseCatalog...))
	}

	allowed := make([]AgentModelOption, 0, len(baseCatalog))
	for _, option := range baseCatalog {
		if h.tierAtLeast(tier, model.Tier(option.MinTier)) {
			allowed = append(allowed, option)
		}
	}
	return h.mergeDynamicModels(ctx, userID, allowed)
}

func (h *AgentHandler) canUseModel(ctx context.Context, userID, modelID, role string, tier model.Tier) bool {
	allowed := h.allowedModels(ctx, userID, role, tier)
	return slices.ContainsFunc(allowed, func(option AgentModelOption) bool {
		return option.ID == modelID
	})
}

func (h *AgentHandler) mergeDynamicModels(ctx context.Context, userID string, catalog []AgentModelOption) []AgentModelOption {
	catalog = appendAgentModelOption(catalog, h.configuredAgentModelOption())
	catalog = appendAgentModelOption(catalog, h.userPreferredModelOption(ctx, userID))
	return catalog
}

func (h *AgentHandler) configuredAgentModelOption() *AgentModelOption {
	if h == nil || h.cfg == nil {
		return nil
	}
	provider := h.cfg.ResolveAgentProvider()
	if provider == nil {
		return nil
	}
	modelID := strings.TrimSpace(provider.Model)
	if modelID == "" {
		return nil
	}
	option := baseAgentModelOption(modelID)
	if option.Label == modelID {
		option.Description = "当前后端 Agent 配置模型"
	}
	return &option
}

func (h *AgentHandler) userPreferredModelOption(ctx context.Context, userID string) *AgentModelOption {
	if userID == "" || h.userSettings == nil || !h.userSettings.IsEnabled() {
		return nil
	}

	settings, err := h.userSettings.GetSettings(ctx, userID)
	if err != nil {
		h.logger.Warn("load user preferred agent model failed", zap.String("user_id", userID), zap.Error(err))
		return nil
	}

	modelID := strings.TrimSpace(settings[storemodel.UserSettingKeyPreferredAIModel])
	if modelID == "" {
		return nil
	}
	if !h.modelAllowedByConfiguredProvider(modelID) {
		return nil
	}

	option := baseAgentModelOption(modelID)
	if preferredName := strings.TrimSpace(settings[storemodel.UserSettingKeyPreferredAIModelName]); preferredName != "" {
		option.Label = preferredName
	}
	if option.Description == "当前后端可用模型" {
		option.Description = "当前用户默认 AI 模型"
	}
	return &option
}

func appendAgentModelOption(catalog []AgentModelOption, option *AgentModelOption) []AgentModelOption {
	if option == nil || strings.TrimSpace(option.ID) == "" {
		return catalog
	}
	if slices.ContainsFunc(catalog, func(existing AgentModelOption) bool {
		return existing.ID == option.ID
	}) {
		return catalog
	}
	return append(catalog, *option)
}

func baseAgentModelOption(modelID string) AgentModelOption {
	trimmedID := strings.TrimSpace(modelID)
	if option, ok := agentModelMetadata[trimmedID]; ok {
		return option
	}
	return AgentModelOption{
		ID:          trimmedID,
		Label:       trimmedID,
		Description: "当前后端可用模型",
		MinTier:     string(model.TierFree),
	}
}

func (h *AgentHandler) providerModelCatalog() []AgentModelOption {
	if h == nil || h.cfg == nil {
		return nil
	}

	providerCfg := h.cfg.ResolveAgentProvider()
	if providerCfg == nil {
		return nil
	}

	provider := strings.ToLower(strings.TrimSpace(providerCfg.Provider))
	configuredModels := normalizeModelList(providerCfg.Models)
	if len(configuredModels) == 0 {
		configuredModels = llm.StandardModels(provider)
	}
	if len(configuredModels) == 0 {
		if configured := h.configuredAgentModelOption(); configured != nil {
			return []AgentModelOption{*configured}
		}
		return nil
	}

	catalog := make([]AgentModelOption, 0, len(configuredModels)+1)
	for _, modelID := range configuredModels {
		option := baseAgentModelOption(modelID)
		catalog = appendAgentModelOption(catalog, &option)
	}
	catalog = appendAgentModelOption(catalog, h.configuredAgentModelOption())
	return catalog
}

func (h *AgentHandler) modelAllowedByConfiguredProvider(modelID string) bool {
	if h == nil || h.cfg == nil {
		return true
	}

	providerCfg := h.cfg.ResolveAgentProvider()
	if providerCfg == nil {
		return true
	}
	provider := strings.ToLower(strings.TrimSpace(providerCfg.Provider))
	if provider == "" {
		return true
	}

	trimmedModelID := strings.TrimSpace(modelID)
	configuredModel := strings.TrimSpace(providerCfg.Model)
	if trimmedModelID != "" && configuredModel != "" && trimmedModelID == configuredModel {
		return true
	}

	configuredModels := normalizeModelList(providerCfg.Models)
	if len(configuredModels) > 0 {
		return slices.Contains(configuredModels, trimmedModelID)
	}

	return slices.Contains(llm.StandardModels(provider), trimmedModelID)
}

func normalizeModelList(raw []string) []string {
	normalized := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))

	for _, entry := range raw {
		modelID := strings.TrimSpace(entry)
		if modelID == "" {
			continue
		}
		if _, exists := seen[modelID]; exists {
			continue
		}
		seen[modelID] = struct{}{}
		normalized = append(normalized, modelID)
	}

	return normalized
}

func (h *AgentHandler) tierAtLeast(current, required model.Tier) bool {
	order := map[model.Tier]int{
		model.TierFree:       0,
		model.TierBasic:      1,
		model.TierStandard:   2,
		model.TierPro:        3,
		model.TierEnterprise: 4,
	}
	return order[current] >= order[required]
}

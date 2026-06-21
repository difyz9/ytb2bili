package handler

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zolagz/ytb2bili/internal/middleware"
	"github.com/zolagz/ytb2bili/internal/service"
	"go.uber.org/zap"
)

// AgentOpenHandler provides the first-stage external agent API surface.
type AgentOpenHandler struct {
	service      *service.AgentOpenService
	videoProcess *VideoProcessHandler
	logger       *zap.Logger
}

func NewAgentOpenHandler(agentOpen *service.AgentOpenService, videoProcess *VideoProcessHandler, logger *zap.Logger) *AgentOpenHandler {
	return &AgentOpenHandler{service: agentOpen, videoProcess: videoProcess, logger: logger}
}

func (h *AgentOpenHandler) RegisterRoutes(r *gin.Engine) {
	if h.service == nil || !h.service.Enabled() {
		h.logger.Info("agent open api disabled; routes not registered")
		return
	}

	api := r.Group("/agent/v1")
	api.Use(middleware.APIKeyAuthMiddleware(h.service))
	api.GET("/capabilities", h.Capabilities)
	api.GET("/tools", middleware.RequireAgentScopes("tools.read"), h.ListTools)
	api.GET("/tools/:tool_name", middleware.RequireAgentScopes("tools.read"), h.GetTool)
	api.POST("/tools/:tool_name/invoke", middleware.RequireAgentScopes("tools.invoke"), h.InvokeTool)
	api.POST("/jobs", middleware.RequireAgentScopes("jobs.create"), h.CreateJob)
	api.GET("/jobs/:job_id", middleware.RequireAgentScopes("jobs.read"), h.GetJob)
	api.GET("/jobs/:job_id/events", middleware.RequireAgentScopes("jobs.read"), h.JobEvents)
	api.GET("/resources/:resource_id", middleware.RequireAgentScopes("resources.read"), h.GetResource)
	api.POST("/resources/:resource_id/signed-url", middleware.RequireAgentScopes("resources.read"), h.CreateSignedURL)
	api.POST("/webhooks/test", middleware.RequireAgentScopes("webhooks.write"), h.TestWebhook)

	h.logger.Info("agent open api routes registered", zap.String("prefix", "/agent/v1"))
}

func (h *AgentOpenHandler) Capabilities(c *gin.Context) {
	Success(c, h.service.Capabilities())
}

func (h *AgentOpenHandler) ListTools(c *gin.Context) {
	mode := c.DefaultQuery("mode", "all")
	tools := h.service.ListTools(mode)
	Success(c, gin.H{"list": tools, "total": len(tools)})
}

func (h *AgentOpenHandler) GetTool(c *gin.Context) {
	toolName := strings.TrimSpace(c.Param("tool_name"))
	tool, ok := h.service.GetTool(toolName)
	if !ok {
		NotFound(c, "resource_not_found")
		return
	}
	Success(c, tool)
}

func (h *AgentOpenHandler) InvokeTool(c *gin.Context) {
	toolName := strings.TrimSpace(c.Param("tool_name"))
	tool, ok := h.service.GetTool(toolName)
	if !ok {
		NotFound(c, "resource_not_found")
		return
	}
	if tool.Mode != service.AgentOpenModeSync {
		BadRequest(c, "invalid_request")
		return
	}
	var req struct {
		Input   map[string]any                  `json:"input" binding:"required"`
		Context service.AgentOpenInvocationContext `json:"context"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid_request")
		return
	}
	principal := middleware.GetAgentOpenPrincipal(c)
	output, credits, err := h.service.InvokeTool(c.Request.Context(), principal, toolName, req.Input, req.Context)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrToolNotFound):
			NotFound(c, "resource_not_found")
		case errors.Is(err, service.ErrToolNotImplemented):
			NotImplemented(c, "tool invocation will be implemented in the next phase")
		default:
			BadRequest(c, "invalid_request")
		}
		return
	}
	Success(c, gin.H{
		"tool_name": toolName,
		"mode":      service.AgentOpenModeSync,
		"output":    output,
		"usage": gin.H{
			"credits":     credits,
			"duration_ms": 0,
		},
	})
}

func (h *AgentOpenHandler) CreateJob(c *gin.Context) {
	var req service.AgentOpenJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid_request")
		return
	}
	principal := middleware.GetAgentOpenPrincipal(c)
	job, err := h.service.CreateJob(c.Request.Context(), principal, req, c.GetHeader("X-Request-Id"), c.GetHeader("Idempotency-Key"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrToolNotFound):
			NotFound(c, "resource_not_found")
		default:
			InternalServerError(c, "internal_error")
		}
		return
	}
	if h.videoProcess != nil {
		h.videoProcess.StartAgentOpenJob(job, req)
	}
	Created(c, gin.H{
		"job_id":     job.JobID,
		"tool_name":  job.ToolName,
		"status":     job.Status,
		"progress":   job.Progress,
		"stage":      job.Stage,
		"created_at": job.CreatedAt,
		"updated_at": job.UpdatedAt,
	})
}

func (h *AgentOpenHandler) GetJob(c *gin.Context) {
	principal := middleware.GetAgentOpenPrincipal(c)
	job, err := h.service.GetJob(c.Request.Context(), principal, c.Param("job_id"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrJobNotFound):
			NotFound(c, "resource_not_found")
		default:
			InternalServerError(c, "internal_error")
		}
		return
	}
	var result any
	if strings.TrimSpace(job.ResultJSON) != "" {
		_ = json.Unmarshal([]byte(job.ResultJSON), &result)
	}
	Success(c, gin.H{
		"job_id":      job.JobID,
		"tool_name":   job.ToolName,
		"status":      job.Status,
		"progress":    job.Progress,
		"stage":       job.Stage,
		"created_at":  job.CreatedAt,
		"updated_at":  job.UpdatedAt,
		"error":       job.ErrorMessage,
		"error_code":  job.ErrorCode,
		"result":      result,
		"credits_spent": job.CreditsSpent,
	})
}

func (h *AgentOpenHandler) JobEvents(c *gin.Context) {
	NotImplemented(c, "job event streaming will be implemented in the next phase")
}

func (h *AgentOpenHandler) GetResource(c *gin.Context) {
	NotImplemented(c, "resource retrieval will be implemented in the next phase")
}

func (h *AgentOpenHandler) CreateSignedURL(c *gin.Context) {
	NotImplemented(c, "signed url generation will be implemented in the next phase")
}

func (h *AgentOpenHandler) TestWebhook(c *gin.Context) {
	var req struct {
		URL string `json:"url" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid_request")
		return
	}
	if _, err := url.ParseRequestURI(req.URL); err != nil {
		BadRequest(c, "invalid_request")
		return
	}
	Success(c, gin.H{
		"reachable":   false,
		"status_code": 0,
		"message":     "webhook probing will be implemented in the next phase",
	})
}
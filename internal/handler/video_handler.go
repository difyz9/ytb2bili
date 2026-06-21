package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zolagz/ytb2bili/internal/analytics"
	"github.com/zolagz/ytb2bili/internal/config"
	"github.com/zolagz/ytb2bili/internal/service"
	"github.com/zolagz/ytb2bili/internal/workflow"
	bili "github.com/zolagz/ytb2bili/pkg/bilibili"
	"github.com/zolagz/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
)

// VideoHandler 视频相关 REST API
// 业务逻辑在 service 层，handler 只负责 HTTP 请求/响应。
type VideoHandler struct {
	logger           *zap.Logger
	videoService     *service.VideoService
	youtubeChain     *workflow.YouTubeChain         // 用于续跑/重试
	biliChain        *workflow.BilibiliChain         // 用于上传
	processingSvc    *workflow.ProcessingService
	bili             *bili.Service
	analytics        *analytics.Client
	userSettings     *service.UserSettingsClient
	cfg              *config.AppConfig
}

func NewVideoHandler(
	logger *zap.Logger,
	videoService *service.VideoService,
	youtubeChain *workflow.YouTubeChain,
	biliChain *workflow.BilibiliChain,
	processingSvc *workflow.ProcessingService,
	biliService *bili.Service,
	analyticsClient *analytics.Client,
	userSettings *service.UserSettingsClient,
	cfg *config.AppConfig,
) *VideoHandler {
	return &VideoHandler{
		logger:        logger,
		videoService:  videoService,
		youtubeChain:  youtubeChain,
		biliChain:     biliChain,
		processingSvc: processingSvc,
		bili:          biliService,
		analytics:     analyticsClient,
		userSettings:  userSettings,
		cfg:           cfg,
	}
}

// ── Route registration ───────────────────────────────────────────────────────

func (h *VideoHandler) RegisterRoutes(r *gin.Engine) {
	h.RegisterRoutesWithAuth(r, nil)
}

func (h *VideoHandler) RegisterRoutesWithAuth(r *gin.Engine, authMid gin.HandlerFunc) {
	api := r.Group("/api/v1/videos")
	{
		api.POST("", h.createVideo)
		api.GET("", h.listVideos)
		api.GET("counts", h.taskCounts)
		api.GET(":id", h.getVideo)
		api.GET(":id/events", h.StreamVideoEvents)
		api.GET(":id/file", h.serveVideoFile)
		api.PUT(":id", h.updateVideo)
		api.DELETE(":id", h.deleteVideo)
	}
	authGroup := r.Group("/api/v1/videos")
	if authMid != nil {
		authGroup.Use(authMid)
	}
	authGroup.POST(":id/steps/:stepName/retry", h.retryTaskStep)
	authGroup.POST(":id/upload-bilibili", h.uploadToBilibili)
	authGroup.POST(":id/resume", h.resumeVideo)
	authGroup.POST(":id/stop", h.stopVideo)
}

// ── CRUD ─────────────────────────────────────────────────────────────────────

func (h *VideoHandler) createVideo(c *gin.Context) {
	var req model.Video
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.videoService.Create(c.Request.Context(), &req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	Success(c, req)
}

func (h *VideoHandler) taskCounts(c *gin.Context) {
	userID := c.Query("user_id")
	sourceType := c.Query("source_type")

	counts, err := h.videoService.CountByTab(c.Request.Context(), userID, sourceType)
	if err != nil {
		h.logger.Error("查询视频统计失败", zap.Error(err))
		InternalServerError(c, "查询视频统计失败")
		return
	}
	Success(c, counts)
}

func (h *VideoHandler) listVideos(c *gin.Context) {
	userID := c.Query("user_id")
	sourceType := c.Query("source_type")
	tab := c.Query("tab")
	page, size := service.ParsePageSize(
		c.DefaultQuery("page", "1"),
		c.DefaultQuery("size", c.DefaultQuery("limit", "10")),
		10,
	)

	videos, total, totalPages, err := h.videoService.List(c.Request.Context(), userID, sourceType, tab, page, size)
	if err != nil {
		h.logger.Error("查询视频列表失败", zap.Error(err))
		InternalServerError(c, "查询视频列表失败")
		return
	}

	Success(c, service.VideoListResponse{
		Videos:     videos,
		Total:      total,
		Page:       page,
		Size:       size,
		TotalPages: totalPages,
	})
}

func (h *VideoHandler) getVideo(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的id")
		return
	}

	video, steps, err := h.videoService.GetWithSteps(c.Request.Context(), uint(id))
	if err != nil {
		if strings.Contains(err.Error(), "不存在") {
			NotFound(c, err.Error())
			return
		}
		h.logger.Error("查询视频失败", zap.Error(err))
		InternalServerError(c, "查询视频失败")
		return
	}

	// Ensure userID from JWT context
	if video.UserID == "" {
		video.UserID = c.GetString("uid")
	}

	Success(c, service.VideoWithStepsWrapper{Video: *video, TaskSteps: steps})
}

func (h *VideoHandler) updateVideo(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的id")
		return
	}

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数错误: "+err.Error())
		return
	}

	if err := h.videoService.Update(c.Request.Context(), uint(id), req); err != nil {
		h.logger.Error("更新视频失败", zap.Error(err))
		InternalServerError(c, "更新视频失败")
		return
	}
	Success(c, nil)
}

func (h *VideoHandler) deleteVideo(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的id")
		return
	}
	if err := h.videoService.Delete(c.Request.Context(), uint(id)); err != nil {
		h.logger.Error("删除视频失败", zap.Error(err))
		InternalServerError(c, "删除视频失败")
		return
	}
	Success(c, nil)
}

// ── File serving ─────────────────────────────────────────────────────────────

func (h *VideoHandler) serveVideoFile(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的id")
		return
	}

	video, err := h.videoService.GetByPrimaryKey(c.Request.Context(), uint(id))
	if err != nil {
		NotFound(c, "视频不存在")
		return
	}
	if video.VideoPath == "" {
		NotFound(c, "视频文件不存在")
		return
	}
	c.File(video.VideoPath)
}

// ── Step management ──────────────────────────────────────────────────────────

type resumeRequest struct {
	PreferredResolution   string                          `json:"preferred_resolution"`
	TaskChainSettings     *workflow.TaskChainSettings     `json:"task_chain_settings"`
	SpeechSynthesisConfig *workflow.SpeechSynthesisConfig `json:"speech_synthesis_config"`
	TranslationConfig     *workflow.TranslationConfig     `json:"translation_config"`
	RestartFromStep       string                          `json:"restart_from_step"`
}

func buildResumeVideoContext(req resumeRequest) *workflow.VideoContext {
	if strings.TrimSpace(req.PreferredResolution) == "" &&
		req.TaskChainSettings == nil &&
		req.SpeechSynthesisConfig == nil &&
		req.TranslationConfig == nil &&
		strings.TrimSpace(req.RestartFromStep) == "" {
		return nil
	}

	return &workflow.VideoContext{
		PreferredResolution:   strings.TrimSpace(req.PreferredResolution),
		TaskChainSettings:     workflow.NormalizeTaskChainSettings(req.TaskChainSettings),
		SpeechSynthesisConfig: req.SpeechSynthesisConfig,
		TranslationConfig:     req.TranslationConfig,
		RestartFromStep:       strings.TrimSpace(req.RestartFromStep),
	}
}

type uploadBilibiliReq struct {
	AccountID   uint   `json:"account_id"`
	Copyright   int    `json:"copyright"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Tags        string `json:"tags"`
	Cover       string `json:"cover"`
}

func (h *VideoHandler) retryTaskStep(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的id")
		return
	}

	stepName := c.Param("stepName")
	if stepName == "" {
		BadRequest(c, "步骤名称不能为空")
		return
	}

	var req resumeRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if strings.TrimSpace(req.RestartFromStep) == "" {
		req.RestartFromStep = stepName
	}

	video, err := h.videoService.GetByPrimaryKey(c.Request.Context(), uint(id))
	if err != nil {
		NotFound(c, "视频不存在")
		return
	}

	// Reset steps from restart point
	if err := h.videoService.ResetStepsFrom(c.Request.Context(), video.VideoID, req.RestartFromStep); err != nil {
		BadRequest(c, "重置步骤失败: "+err.Error())
		return
	}

	if video.UserID == "" {
		video.UserID = c.GetString("uid")
	}

	// Start async retry
	resumeCtx := buildResumeVideoContext(req)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		var err error
		if resumeCtx != nil {
			err = h.youtubeChain.ResumeProcessingWithContext(ctx, video, resumeCtx)
		} else {
			err = h.youtubeChain.ResumeProcessing(ctx, video)
		}
		if err != nil {
			h.logger.Error("单步重试续跑失败",
				zap.String("video_id", video.VideoID),
				zap.String("step_name", req.RestartFromStep),
				zap.Error(err))
		}
	}()

	Success(c, gin.H{"message": "步骤重试已启动"})
}

func (h *VideoHandler) resumeVideo(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的id")
		return
	}

	var req resumeRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		BadRequest(c, "参数错误: "+err.Error())
		return
	}

	video, err := h.videoService.GetByPrimaryKey(c.Request.Context(), uint(id))
	if err != nil {
		NotFound(c, "视频不存在")
		return
	}
	if video.UserID == "" {
		video.UserID = c.GetString("uid")
	}

	if strings.TrimSpace(req.RestartFromStep) != "" {
		if err := h.videoService.ResetStepsFrom(c.Request.Context(), video.VideoID, req.RestartFromStep); err != nil {
			BadRequest(c, "重置步骤失败: "+err.Error())
			return
		}
	}

	// Mark as processing
	h.videoService.MarkStatus(c.Request.Context(), video.VideoID, model.VideoStatusProcessing)
	Success(c, gin.H{"message": "重新处理已启动"})

	resumeCtx := buildResumeVideoContext(req)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		var err error
		if resumeCtx != nil {
			err = h.youtubeChain.ResumeProcessingWithContext(ctx, video, resumeCtx)
		} else {
			err = h.youtubeChain.ResumeProcessing(ctx, video)
		}
		if err != nil {
			h.logger.Error("重新处理失败", zap.String("video_id", video.VideoID), zap.Error(err))
		}
	}()
}

func (h *VideoHandler) stopVideo(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的id")
		return
	}

	video, err := h.videoService.GetByPrimaryKey(c.Request.Context(), uint(id))
	if err != nil {
		NotFound(c, "视频不存在")
		return
	}

	// Cancel via processing service
	if err := h.processingSvc.CancelTask(video.VideoID); err != nil {
		BadRequest(c, err.Error())
		return
	}

	h.videoService.MarkStatus(c.Request.Context(), video.VideoID, model.VideoStatusPaused)
	h.videoService.MarkStepsStopping(c.Request.Context(), video.VideoID)
	Success(c, gin.H{"message": "停止请求已发送"})
}

// ── SSE streaming ────────────────────────────────────────────────────────────

func (h *VideoHandler) StreamVideoEvents(c *gin.Context) {
	videoID := c.Param("id")

	// Resolve video
	_, err := h.videoService.GetByVideoID(c.Request.Context(), videoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "视频不存在"})
		return
	}

	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, canFlush := c.Writer.(http.Flusher)

	loadSteps := func() []model.TaskStep {
		steps, _ := h.videoService.ListSteps(c.Request.Context(), videoID)
		return steps
	}

	reloadVideo := func() (status, bvid string) {
		v, _ := h.videoService.GetByVideoID(context.Background(), videoID)
		if v != nil {
			return v.Status, v.BiliBVID
		}
		return "", ""
	}

	type stepKey struct{ name, status string }
	sent := make(map[stepKey]bool)

	sendSteps := func(steps []model.TaskStep) {
		for _, s := range steps {
			k := stepKey{s.StepName, s.Status}
			if sent[k] {
				continue
			}
			sent[k] = true
			payload := map[string]interface{}{
				"step_name":  s.StepName,
				"status":     s.Status,
				"step_order": s.StepOrder,
			}
			if s.Duration > 0 {
				payload["duration"] = s.Duration
			}
			if s.ProgressPercent > 0 {
				payload["progress_percent"] = s.ProgressPercent
			}
			if s.ProgressText != "" {
				payload["progress_text"] = s.ProgressText
			}
			if s.ErrorMsg != "" {
				payload["error_msg"] = s.ErrorMsg
			}
			writeSSEEvent(c.Writer, payload) //nolint:errcheck
		}
		if canFlush {
			flusher.Flush()
		}
	}

	ctx := c.Request.Context()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	sendSteps(loadSteps())
	status, bvid := reloadVideo()
	if sseTerminalStatus(status) {
		writeSSEEvent(c.Writer, map[string]interface{}{ //nolint:errcheck
			"type":         "done",
			"video_status": status,
			"bili_bvid":    bvid,
		})
		if canFlush {
			flusher.Flush()
		}
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sendSteps(loadSteps())
			status, bvid = reloadVideo()
			if sseTerminalStatus(status) {
				writeSSEEvent(c.Writer, map[string]interface{}{ //nolint:errcheck
					"type":         "done",
					"video_status": status,
					"bili_bvid":    bvid,
				})
				if canFlush {
					flusher.Flush()
				}
				return
			}
		}
	}
}

// ── Bilibili upload ──────────────────────────────────────────────────────────

func (h *VideoHandler) uploadToBilibili(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的id")
		return
	}

	var req uploadBilibiliReq
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		BadRequest(c, "参数错误: "+err.Error())
		return
	}

	video, err := h.videoService.GetByPrimaryKey(c.Request.Context(), uint(id))
	if err != nil {
		NotFound(c, "视频不存在")
		return
	}

	if video.VideoPath == "" {
		BadRequest(c, "视频文件路径为空，无法上传")
		return
	}

	if video.BiliBVID != "" {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "该视频已上传到B站",
			"data": gin.H{
				"bvid":      video.BiliBVID,
				"aid":       video.BiliAID,
				"video_url": "https://www.bilibili.com/video/" + video.BiliBVID,
			},
		})
		return
	}

	userID := video.UserID
	if userID == "" {
		userID = c.GetString("uid")
	}

	overrides := &workflow.BilibiliSubmissionOverrides{
		AccountID:   req.AccountID,
		Copyright:   req.Copyright,
		Title:       strings.TrimSpace(req.Title),
		Description: strings.TrimSpace(req.Description),
		Tags:        strings.TrimSpace(req.Tags),
		Cover:       strings.TrimSpace(req.Cover),
	}
	if overrides.AccountID == 0 && overrides.Copyright == 0 &&
		overrides.Title == "" && overrides.Description == "" &&
		overrides.Tags == "" && overrides.Cover == "" {
		overrides = nil
	}

	result, uploadErr := uploadVideoToBilibili(c.Request.Context(), h.logger, h.bili, h.biliChain, h.analytics, userID, video, overrides)
	if uploadErr != nil {
		h.logger.Error("手动上传B站失败", zap.Uint("id", video.ID), zap.String("video_id", video.VideoID), zap.Error(uploadErr))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "上传失败: " + uploadErr.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "上传成功",
		"data": gin.H{
			"bvid":      result.BiliBVID,
			"aid":       result.BiliAID,
			"video_url": "https://www.bilibili.com/video/" + result.BiliBVID,
		},
	})
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func sseTerminalStatus(status string) bool {
	switch status {
	case "003", "completed", "004", "failed", model.VideoStatusPaused:
		return true
	}
	return false
}

func writeSSEEvent(w io.Writer, payload interface{}) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", b)
	return err
}

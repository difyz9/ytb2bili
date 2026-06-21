package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zolagz/ytb2bili/internal/config"
	"github.com/zolagz/ytb2bili/internal/service"
	"github.com/zolagz/ytb2bili/internal/workflow"
	"github.com/zolagz/ytb2bili/pkg/utils"

	"github.com/zolagz/ytb2bili/pkg/store/model"
	"github.com/zolagz/ytb2bili/pkg/tools"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// VideoProcessHandler 视频处理处理器
// 业务逻辑已移至 ProcessingService，handler 只负责 HTTP 请求/响应 + 文件上传。
type VideoProcessHandler struct {
	logger            *zap.Logger
	videoService      *service.VideoService
	processingSvc     *workflow.ProcessingService
	downloadVideoTool *tools.DownloadVideoTool
	userSettings      *service.UserSettingsClient
	cfg               *config.AppConfig
}

type VideoProcessHandlerParams struct {
	fx.In
	VideoService      *service.VideoService
	ProcessingSvc     *workflow.ProcessingService
	Logger            *zap.Logger
	DownloadVideoTool *tools.DownloadVideoTool `optional:"true"`
	UserSettings      *service.UserSettingsClient
	Cfg               *config.AppConfig
}

func NewVideoProcessHandler(params VideoProcessHandlerParams) *VideoProcessHandler {
	return &VideoProcessHandler{
		logger:            params.Logger,
		videoService:      params.VideoService,
		processingSvc:     params.ProcessingSvc,
		downloadVideoTool: params.DownloadVideoTool,
		userSettings:      params.UserSettings,
		cfg:               params.Cfg,
	}
}

// ── Request / Response types ─────────────────────────────────────────────────

type SubmitLinkRequest struct {
	URL                   string                          `json:"url" binding:"required"`
	UserID                string                          `json:"user_id"`
	PreferredResolution   string                          `json:"preferred_resolution"`
	TaskChainSettings     *workflow.TaskChainSettings     `json:"task_chain_settings"`
	SpeechSynthesisConfig *workflow.SpeechSynthesisConfig `json:"speech_synthesis_config"`
	PlaylistConfig        *PlaylistSubmitConfig           `json:"playlist_config"`
}

type PlaylistSubmitConfig struct {
	Enabled    bool `json:"enabled"`
	StartIndex int  `json:"start_index"`
	MaxItems   int  `json:"max_items"`
}

type SubmitVideoRequest struct {
	VideoPath             string                          `json:"video_path" binding:"required"`
	UserID                string                          `json:"user_id"`
	Title                 string                          `json:"title"`
	PreferredResolution   string                          `json:"preferred_resolution"`
	TaskChainSettings     *workflow.TaskChainSettings     `json:"task_chain_settings"`
	SpeechSynthesisConfig *workflow.SpeechSynthesisConfig `json:"speech_synthesis_config"`
}

type VideoProcessResponse struct {
	Success bool              `json:"success"`
	Message string            `json:"message"`
	Data    *VideoProcessData `json:"data,omitempty"`
}

type VideoProcessData struct {
	VideoID     string                  `json:"video_id"`
	VideoPath   string                  `json:"video_path"`
	AudioPath   string                  `json:"audio_path,omitempty"`
	Transcript  *tools.TranscriptResult `json:"transcript,omitempty"`
	Status      string                  `json:"status"`
	ProcessedAt time.Time               `json:"processed_at"`
}

type UploadVideoResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	VideoID   string `json:"video_id,omitempty"`
	VideoPath string `json:"video_path,omitempty"`
	FileName  string `json:"file_name,omitempty"`
}

const (
	defaultPlaylistStartIndex = 1
	defaultPlaylistMaxItems   = 10
	maxPlaylistBatchItems     = 50
)

// ── Route registration ───────────────────────────────────────────────────────

func (h *VideoProcessHandler) RegisterRoutesWithAuth(r *gin.Engine, jwtMid gin.HandlerFunc) {
	api := r.Group("/api/v1/video-process")
	if jwtMid != nil {
		api.Use(jwtMid)
	}
	api.POST("/submit-link", h.SubmitLink)
	api.POST("/submit-video", h.SubmitVideo)
	api.POST("/upload", h.UploadVideo)
	api.POST("/async-submit-link", h.AsyncSubmitLink)
}

// ── SubmitLink (sync) ────────────────────────────────────────────────────────

func (h *VideoProcessHandler) SubmitLink(c *gin.Context) {
	var req SubmitLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, VideoProcessResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	if uid := c.GetString("uid"); uid != "" {
		req.UserID = uid
	}

	platform, normalizedURL, videoID, douyinInfo, resolveErr := h.processingSvc.ResolveRemoteVideoTarget(c.Request.Context(), req.URL)
	if resolveErr != nil {
		c.JSON(http.StatusBadRequest, VideoProcessResponse{Success: false, Message: resolveErr.Error()})
		return
	}

	settings := workflow.NormalizeTaskChainSettings(req.TaskChainSettings)
	speechCfg := req.SpeechSynthesisConfig

	// Build VideoContext and process
	translationCfg := h.resolveTranslationConfig(c.Request.Context(), req.UserID)
	initialCtx := &workflow.VideoContext{
		Platform:              platform,
		VideoURL:              normalizedURL,
		UserID:                req.UserID,
		VideoID:               videoID,
		PreferredResolution:   req.PreferredResolution,
		TranslationConfig:     translationCfg,
		SpeechSynthesisConfig: speechCfg,
		TaskChainSettings:     settings,
	}

	var result *workflow.VideoContext
	var err error
	if platform == "douyin" {
		initialCtx.DouyinVideoInfo = douyinInfo
		result, err = h.processingSvc.DouyinChain().ProcessContextWithTracking(c.Request.Context(), initialCtx, videoID, req.UserID)
	} else {
		result, err = h.processingSvc.YouTubeChain().ProcessContextWithTracking(c.Request.Context(), initialCtx, videoID, req.UserID)
	}
	if err != nil {
		h.videoService.MarkStatus(c.Request.Context(), videoID, model.VideoStatusFailed)
		c.JSON(http.StatusInternalServerError, VideoProcessResponse{Success: false, Message: "视频处理失败: " + err.Error()})
		return
	}

	// Save results to DB
	updates := map[string]interface{}{
		"platform": platform, "title": result.Title, "description": result.Description,
		"status": model.VideoStatusCompleted, "video_path": result.VideoPath,
	}
	if result.Transcript != nil {
		updates["subtitle_path"] = result.Transcript.SRTPath
	}
	h.videoService.UpdateProcessingResult(c.Request.Context(), result.VideoID, updates)

	c.JSON(http.StatusOK, VideoProcessResponse{
		Success: true, Message: "视频处理成功",
		Data: &VideoProcessData{
			VideoID: result.VideoID, VideoPath: result.VideoPath,
			AudioPath: result.AudioPath, Transcript: result.Transcript,
			Status: "processed", ProcessedAt: time.Now(),
		},
	})
}

// ── SubmitVideo (local file) ─────────────────────────────────────────────────

func (h *VideoProcessHandler) SubmitVideo(c *gin.Context) {
	var req SubmitVideoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, VideoProcessResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	if uid := c.GetString("uid"); uid != "" {
		req.UserID = uid
	}

	// Path traversal check
	downloadDir := h.cfg.Workflow.DownloadDir
	if downloadDir == "" {
		downloadDir = "./downloads"
	}
	absAllowedDir, _ := filepath.Abs(downloadDir)
	absVideoPath, _ := filepath.Abs(req.VideoPath)
	if absVideoPath == "" || !isSubPath(absAllowedDir, absVideoPath) {
		c.JSON(http.StatusBadRequest, VideoProcessResponse{Success: false, Message: "视频路径不在允许的目录内"})
		return
	}
	if _, err := os.Stat(absVideoPath); err != nil {
		c.JSON(http.StatusBadRequest, VideoProcessResponse{Success: false, Message: "视频文件不存在: " + req.VideoPath})
		return
	}

	videoID := extractVideoIDFromPath(req.VideoPath)
	settings := workflow.NormalizeTaskChainSettings(req.TaskChainSettings)
	speechCfg := req.SpeechSynthesisConfig
	translationCfg := h.resolveTranslationConfig(c.Request.Context(), req.UserID)

	// Process local video via YouTube chain
	initialCtx := &workflow.VideoContext{
		VideoID: videoID, VideoPath: req.VideoPath, Title: req.Title, UserID: req.UserID,
		TranslationConfig: translationCfg, SpeechSynthesisConfig: speechCfg,
		TaskChainSettings: settings,
	}
	result, err := h.processingSvc.YouTubeChain().ProcessLocalContextWithTracking(c.Request.Context(), initialCtx, videoID, req.UserID)
	if err != nil {
		h.videoService.MarkStatus(c.Request.Context(), videoID, model.VideoStatusFailed)
		c.JSON(http.StatusInternalServerError, VideoProcessResponse{Success: false, Message: "视频处理失败: " + err.Error()})
		return
	}

	updates := map[string]interface{}{"status": model.VideoStatusCompleted, "video_path": result.VideoPath}
	if result.Title != "" {
		updates["title"] = result.Title
	}
	if result.Transcript != nil {
		updates["subtitle_path"] = result.Transcript.SRTPath
	}
	h.videoService.UpdateProcessingResult(c.Request.Context(), videoID, updates)

	c.JSON(http.StatusOK, VideoProcessResponse{
		Success: true, Message: "视频处理成功",
		Data: &VideoProcessData{
			VideoID: videoID, VideoPath: result.VideoPath,
			AudioPath: result.AudioPath, Transcript: result.Transcript,
			Status: "processed", ProcessedAt: time.Now(),
		},
	})
}

// ── UploadVideo (file upload) ────────────────────────────────────────────────

func (h *VideoProcessHandler) UploadVideo(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, UploadVideoResponse{Success: false, Message: "获取文件失败: " + err.Error()})
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowedExts := map[string]bool{".mp4": true, ".avi": true, ".mov": true, ".mkv": true, ".flv": true, ".wmv": true, ".webm": true}
	if !allowedExts[ext] {
		c.JSON(http.StatusBadRequest, UploadVideoResponse{Success: false, Message: "不支持的文件格式: " + ext})
		return
	}

	uploadDir := h.cfg.Workflow.DownloadDir
	if uploadDir == "" {
		uploadDir = "./downloads"
	}
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, UploadVideoResponse{Success: false, Message: "创建上传目录失败"})
		return
	}

	safeFilename := filepath.Base(file.Filename)
	tempPath := filepath.Join(uploadDir, fmt.Sprintf("temp_%d_%s", time.Now().Unix(), safeFilename))
	if err := c.SaveUploadedFile(file, tempPath); err != nil {
		c.JSON(http.StatusInternalServerError, UploadVideoResponse{Success: false, Message: "保存文件失败"})
		return
	}
	defer os.Remove(tempPath)

	videoID, err := calcFileHash(tempPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, UploadVideoResponse{Success: false, Message: "计算文件哈希失败"})
		return
	}

	videoDir := filepath.Join(uploadDir, videoID)
	if err := os.MkdirAll(videoDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, UploadVideoResponse{Success: false, Message: "创建视频目录失败"})
		return
	}

	finalPath := filepath.Join(videoDir, safeFilename)
	if err := os.Rename(tempPath, finalPath); err != nil {
		c.JSON(http.StatusInternalServerError, UploadVideoResponse{Success: false, Message: "移动文件失败"})
		return
	}

	absPath, _ := filepath.Abs(finalPath)
	c.JSON(http.StatusOK, UploadVideoResponse{
		Success: true, Message: "文件上传成功",
		VideoID: videoID, VideoPath: absPath, FileName: safeFilename,
	})
}

// ── AsyncSubmitLink ──────────────────────────────────────────────────────────

func (h *VideoProcessHandler) AsyncSubmitLink(c *gin.Context) {
	var req SubmitLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求参数错误: " + err.Error()})
		return
	}

	if uid := c.GetString("uid"); uid != "" {
		req.UserID = uid
	}

	// Resolve preferences
	settings := workflow.NormalizeTaskChainSettings(req.TaskChainSettings)
	speechCfg := req.SpeechSynthesisConfig

	// Handle playlist
	if shouldExpandPlaylist(req.URL, req.PlaylistConfig) && h.downloadVideoTool != nil {
		playlistCfg := normalizePlaylistConfig(req.PlaylistConfig)
		playlistResult, err := h.downloadVideoTool.ListPlaylistEntries(c.Request.Context(), req.URL, tools.PlaylistOptions{
			Enabled: true, StartIndex: playlistCfg.StartIndex, MaxItems: playlistCfg.MaxItems,
		})
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
			return
		}

		speechVoiceName := ""
		if speechCfg != nil {
			speechVoiceName = speechCfg.VoiceName
		}
		for _, entry := range playlistResult.Entries {
			settingsJSON, _ := json.Marshal(settings)
			h.videoService.UpsertAsProcessing(entry.VideoID, entry.URL, entry.Title,
				req.UserID, "youtube", req.PreferredResolution,
				speechVoiceName, string(settingsJSON), playlistResult.PlaylistID)
			h.processingSvc.EnqueueRemoteVideoProcessing("youtube", entry.URL, entry.VideoID,
				req.UserID, req.PreferredResolution, nil, settings, speechCfg)
		}

		c.JSON(http.StatusOK, gin.H{
			"code": 0, "message": fmt.Sprintf("播放列表已加入处理队列，共 %d 条", len(playlistResult.Entries)),
			"data": gin.H{
				"video_id": playlistResult.Entries[0].VideoID, "playlist_id": playlistResult.PlaylistID,
				"submitted_count": len(playlistResult.Entries), "submission_mode": "playlist",
			},
		})
		return
	}

	// Single video
	platform, normalizedURL, videoID, douyinInfo, resolveErr := h.processingSvc.ResolveRemoteVideoTarget(c.Request.Context(), req.URL)
	if resolveErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": resolveErr.Error()})
		return
	}

	speechVoiceName := ""
	if speechCfg != nil {
		speechVoiceName = speechCfg.VoiceName
	}
	settingsJSON, _ := json.Marshal(settings)
	h.videoService.UpsertAsProcessing(videoID, normalizedURL, "", req.UserID, platform,
		req.PreferredResolution, speechVoiceName, string(settingsJSON), "")
	h.processingSvc.EnqueueRemoteVideoProcessing(platform, normalizedURL, videoID,
		req.UserID, req.PreferredResolution, douyinInfo, settings, speechCfg)

	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "已加入处理队列",
		"data": gin.H{"video_id": videoID, "submitted_count": 1, "submission_mode": "single"},
	})
}

// ── Agent Open API ───────────────────────────────────────────────────────────

func (h *VideoProcessHandler) StartAgentOpenJob(job *model.AgentJob, req service.AgentOpenJobRequest) {
	if job == nil {
		return
	}
	switch job.ToolName {
	case "submit_pipeline":
		h.startAgentOpenPipeline(job, req)
	default:
		h.updateJob(job.JobID, map[string]any{
			"status": "failed", "progress": 100, "stage": "failed",
			"error_code": "not_implemented",
			"error_message": "async tool implementation is not available yet",
		})
	}
}

func (h *VideoProcessHandler) startAgentOpenPipeline(job *model.AgentJob, req service.AgentOpenJobRequest) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		h.updateJob(job.JobID, map[string]any{"status": "running", "progress": 10, "stage": "starting"})

		videoURL := agentOpenStr(req.Input["url"])
		if videoURL == "" {
			h.failJob(job.JobID, "invalid_request", "url is required")
			return
		}

		preferredRes := agentOpenStr(req.Input["preferred_resolution"])
		resolvedResolution := utils.NormalizeResolution(preferredRes)

		platform, normalizedURL, videoID, douyinInfo, err := h.processingSvc.ResolveRemoteVideoTarget(ctx, videoURL)
		if err != nil {
			h.failJob(job.JobID, "unsupported_platform", err.Error())
			return
		}

		h.updateJob(job.JobID, map[string]any{"progress": 20, "stage": "queued_video"})
		h.videoService.UpsertAsProcessing(videoID, normalizedURL, "", job.OwnerUserID, platform,
			resolvedResolution, "", "{}", "")

		h.updateJob(job.JobID, map[string]any{"progress": 30, "stage": "processing_video"})
		_, procErr := h.processingSvc.ProcessRemoteVideo(ctx, platform, normalizedURL, videoID,
			job.OwnerUserID, resolvedResolution, douyinInfo, nil, nil)
		if procErr != nil {
			if errors.Is(procErr, context.Canceled) {
				h.videoService.MarkStatus(ctx, videoID, model.VideoStatusPaused)
				h.updateJob(job.JobID, map[string]any{"status": "cancelled", "progress": 100, "stage": "cancelled"})
				return
			}
			h.videoService.MarkStatus(ctx, videoID, model.VideoStatusFailed)
			h.failJob(job.JobID, "pipeline_failed", procErr.Error())
			return
		}

		resultJSON, _ := json.Marshal(gin.H{
			"video_id": videoID, "platform": platform, "status": model.VideoStatusCompleted,
		})
		h.updateJob(job.JobID, map[string]any{
			"status": "completed", "progress": 100, "stage": "completed", "result_json": string(resultJSON),
		})
	}()
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func (h *VideoProcessHandler) resolveTranslationConfig(ctx context.Context, userID string) *workflow.TranslationConfig {
	return ResolveVideoTranslationConfig(ctx, h.cfg, h.userSettings, userID, nil)
}

func (h *VideoProcessHandler) updateJob(jobID string, updates map[string]any) {
	// The handler has no direct DB access; this is a thin wrapper
	// Agent Open job updates should be done through a dedicated service
}

func (h *VideoProcessHandler) failJob(jobID, errorCode, errorMessage string) {
	h.updateJob(jobID, map[string]any{
		"status": "failed", "progress": 100, "stage": "failed",
		"error_code": errorCode, "error_message": errorMessage,
	})
}

func normalizePlaylistConfig(cfg *PlaylistSubmitConfig) tools.PlaylistOptions {
	if cfg == nil || !cfg.Enabled {
		return tools.PlaylistOptions{}
	}
	startIndex := cfg.StartIndex
	if startIndex < 1 {
		startIndex = defaultPlaylistStartIndex
	}
	maxItems := cfg.MaxItems
	if maxItems <= 0 {
		maxItems = defaultPlaylistMaxItems
	}
	if maxItems > maxPlaylistBatchItems {
		maxItems = maxPlaylistBatchItems
	}
	return tools.PlaylistOptions{Enabled: true, StartIndex: startIndex, MaxItems: maxItems}
}

func shouldExpandPlaylist(url string, cfg *PlaylistSubmitConfig) bool {
	return cfg != nil && cfg.Enabled && tools.ExtractYouTubePlaylistID(url) != ""
}

func calcFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}
	h := hex.EncodeToString(hash.Sum(nil))
	if len(h) >= 12 {
		return h[:12], nil
	}
	return h, nil
}

func isSubPath(parent, child string) bool {
	parent = filepath.Clean(parent) + string(os.PathSeparator)
	return strings.HasPrefix(filepath.Clean(child)+string(os.PathSeparator), parent)
}

func extractVideoIDFromPath(videoPath string) string {
	filename := videoPath
	if idx := strings.LastIndexAny(filename, "/\\"); idx >= 0 {
		filename = filename[idx+1:]
	}
	if idx := strings.LastIndex(filename, "."); idx >= 0 {
		filename = filename[:idx]
	}
	if len(filename) == 11 {
		return filename
	}
	return filename
}


func agentOpenStr(v any) string {
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func agentOpenBool(v any, def bool) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return def
}

func agentOpenMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

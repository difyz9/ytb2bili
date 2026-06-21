package handler

import (
	"net/http"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zolagz/ytb2bili/internal/analytics"
	"github.com/zolagz/ytb2bili/internal/workflow"
	"go.uber.org/zap"
)

// UploadToBilibiliHandler B站上传处理器
type UploadToBilibiliHandler struct {
	biliChain *workflow.BilibiliChain
	logger    *zap.Logger
	analytics *analytics.Client
}

// NewUploadToBilibiliHandler 创建B站上传处理器
func NewUploadToBilibiliHandler(
	biliChain *workflow.BilibiliChain,
	logger *zap.Logger,
	analyticsClient *analytics.Client,
) *UploadToBilibiliHandler {
	return &UploadToBilibiliHandler{
		biliChain: biliChain,
		logger:    logger,
		analytics: analyticsClient,
	}
}

// RegisterRoutes 注册路由
func (h *UploadToBilibiliHandler) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api/v1/upload")
	{
		api.POST("/bilibili", h.uploadToBilibili)
	}
}

// UploadRequest 上传请求
type UploadRequest struct {
	VideoPath string `json:"video_path" binding:"required"` // 视频文件路径
	VideoURL  string `json:"video_url"`                     // 原视频URL（可选）
}

// uploadToBilibili godoc
// @Summary 上传视频到B站
// @Description 将已下载的视频上传到B站（使用用户的主B站账号）
// @Tags upload
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body UploadRequest true "上传请求"
// @Success 200 {object} map[string]interface{} "上传成功"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 500 {object} map[string]interface{} "上传失败"
// @Router /api/v1/upload/bilibili [post]
func (h *UploadToBilibiliHandler) uploadToBilibili(c *gin.Context) {
	// 1. 获取用户ID
	userID, exists := c.Get("uid")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "未授权",
		})
		return
	}

	uid, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "用户ID类型错误",
		})
		return
	}

	// 2. 解析请求
	var req UploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	h.logger.Info("收到B站上传请求",
		zap.String("user_id", uid),
		zap.String("video_path", req.VideoPath),
		zap.String("video_url", req.VideoURL))

	// 3. 执行上传工作流
	result, err := h.biliChain.RunFromVideoPath(
		c.Request.Context(),
		uid,
		req.VideoPath,
		req.VideoURL,
		nil,
	)

	if err != nil {
		h.logger.Error("上传到B站失败",
			zap.Error(err),
			zap.String("user_id", uid),
			zap.String("video_path", req.VideoPath))

		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "上传失败: " + err.Error(),
		})
		return
	}

	// 4. 返回成功结果
	h.logger.Info("上传到B站成功",
		zap.String("user_id", uid),
		zap.String("bvid", result.BiliBVID),
		zap.Int64("aid", result.BiliAID))

	// 5. 上报上传成功事件到 analytics 后端
	if h.analytics != nil {
		go func() {
			properties := map[string]interface{}{
				"user_id":           uid,
				"bvid":              result.BiliBVID,
				"aid":               result.BiliAID,
				"video_id":          result.VideoID,
				"source_url":        req.VideoURL,
				"bili_video_url":    "https://www.bilibili.com/video/" + result.BiliBVID,
				"title":             result.Title,
				"upload_time":       time.Now().Unix(),
				"resource_type":     "video",
				"resource_filename": filepath.Base(req.VideoPath),
			}

			h.analytics.TrackBilibiliVideoUploadSuccess(properties)
			h.logger.Info("视频上传成功事件已上报",
				zap.String("user_id", uid),
				zap.String("bvid", result.BiliBVID))
		}()
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "上传成功",
		"data": gin.H{
			"bvid":      result.BiliBVID,
			"aid":       result.BiliAID,
			"video_id":  result.VideoID,
			"video_url": "https://www.bilibili.com/video/" + result.BiliBVID,
		},
	})
}

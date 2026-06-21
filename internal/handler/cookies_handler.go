package handler

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/zolagz/ytb2bili/internal/config"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type CookiesHandler struct {
	Config *config.AppConfig
	Logger *zap.Logger
}

func NewCookiesHandler(cfg *config.AppConfig, logger *zap.Logger) *CookiesHandler {
	return &CookiesHandler{
		Config: cfg,
		Logger: logger,
	}
}

// RegisterRoutes 注册 cookies 相关路由
func (h *CookiesHandler) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api/v1")

	cookies := api.Group("/cookies")
	{
		cookies.POST("/upload", h.uploadCookies)
		cookies.GET("/status", h.getCookiesStatus)
		cookies.DELETE("/", h.deleteCookies)
	}
}

// CookiesStatusData cookies 状态数据
type CookiesStatusData struct {
	HasCookies bool   `json:"has_cookies"`
	FilePath   string `json:"file_path,omitempty"`
	FileSize   int64  `json:"file_size,omitempty"`
	UpdateTime string `json:"update_time,omitempty"`
}

// uploadCookies 上传 cookies 文件
// @Summary      上传 YouTube cookies 文件
// @Description  上传 Netscape 格式的 cookies.txt 文件，用于 yt-dlp 下载受限制的 YouTube 视频。文件需从浏览器导出，最大支持 10MB
// @Tags         cookies
// @Accept       multipart/form-data
// @Produce      json
// @Param        file  formData  file  true  "Cookies 文件 (Netscape 格式，.txt 文件)"
// @Success      200  {object}  Response{data=CookiesStatusData}  "上传成功"
// @Failure      400  {object}  Response  "参数错误：文件格式不正确、文件过大或文件格式无效"
// @Failure      500  {object}  Response  "服务器错误：文件写入失败"
// @Router       /api/v1/cookies/upload [post]
func (h *CookiesHandler) uploadCookies(c *gin.Context) {
	// 获取上传的文件
	file, err := c.FormFile("file")
	if err != nil {
		BadRequest(c, "未找到上传文件")
		return
	}

	// 验证文件扩展名
	if !strings.HasSuffix(strings.ToLower(file.Filename), ".txt") {
		BadRequest(c, "只支持 .txt 格式的 cookies 文件")
		return
	}

	// 验证文件大小 (最大 10MB)
	if file.Size > 10*1024*1024 {
		BadRequest(c, "文件大小不能超过 10MB")
		return
	}

	// 获取 cookies 文件路径
	cookiesPath := h.getCookiesPath()

	// 确保目录存在
	dir := filepath.Dir(cookiesPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		h.Logger.Error("创建目录失败", zap.Error(err))
		InternalServerError(c, "创建目录失败")
		return
	}

	// 打开上传的文件
	src, err := file.Open()
	if err != nil {
		h.Logger.Error("打开上传文件失败", zap.Error(err))
		InternalServerError(c, "打开上传文件失败")
		return
	}
	defer src.Close()

	// 读取文件内容进行验证
	content, err := io.ReadAll(src)
	if err != nil {
		h.Logger.Error("读取文件内容失败", zap.Error(err))
		InternalServerError(c, "读取文件内容失败")
		return
	}

	// 验证 cookies 文件格式
	if !h.validateCookiesContent(string(content)) {
		BadRequest(c, "无效的 cookies 文件格式，请确保是 Netscape 格式的 cookies.txt")
		return
	}

	// 如果已存在旧文件，先备份
	if _, err := os.Stat(cookiesPath); err == nil {
		backupPath := cookiesPath + ".backup"
		if err := os.Rename(cookiesPath, backupPath); err != nil {
			h.Logger.Warn("备份旧文件失败", zap.Error(err))
		} else {
			h.Logger.Info("已备份旧 cookies 文件", zap.String("path", backupPath))
		}
	}

	// 创建目标文件
	dst, err := os.Create(cookiesPath)
	if err != nil {
		h.Logger.Error("创建目标文件失败", zap.Error(err))
		InternalServerError(c, "创建目标文件失败")
		return
	}
	defer dst.Close()

	// 写入内容
	if _, err := dst.Write(content); err != nil {
		h.Logger.Error("写入文件失败", zap.Error(err))
		InternalServerError(c, "写入文件失败")
		return
	}

	// 设置文件权限
	if err := os.Chmod(cookiesPath, 0600); err != nil {
		h.Logger.Warn("设置文件权限失败", zap.Error(err))
	}

	h.Logger.Info("Cookies 文件上传成功",
		zap.String("path", cookiesPath),
		zap.Int64("size", file.Size),
	)

	// 获取文件信息
	fileInfo, _ := os.Stat(cookiesPath)

	Success(c, CookiesStatusData{
		HasCookies: true,
		FilePath:   cookiesPath,
		FileSize:   fileInfo.Size(),
		UpdateTime: fileInfo.ModTime().Format("2006-01-02 15:04:05"),
	})
}

// getCookiesStatus 获取 cookies 文件状态
// @Summary      获取 cookies 文件状态
// @Description  检查 cookies 文件是否存在，返回文件路径、大小和最后更新时间等详细信息
// @Tags         cookies
// @Accept       json
// @Produce      json
// @Success      200  {object}  Response{data=CookiesStatusData}  "成功返回文件状态"
// @Failure      500  {object}  Response  "检查文件失败"
// @Router       /api/v1/cookies/status [get]
func (h *CookiesHandler) getCookiesStatus(c *gin.Context) {
	cookiesPath := h.getCookiesPath()

	fileInfo, err := os.Stat(cookiesPath)
	if err != nil {
		if os.IsNotExist(err) {
			Success(c, CookiesStatusData{
				HasCookies: false,
			})
		} else {
			InternalServerError(c, fmt.Sprintf("检查文件失败: %v", err))
		}
		return
	}

	Success(c, CookiesStatusData{
		HasCookies: true,
		FilePath:   cookiesPath,
		FileSize:   fileInfo.Size(),
		UpdateTime: fileInfo.ModTime().Format("2006-01-02 15:04:05"),
	})
}

// deleteCookies 删除 cookies 文件
// @Summary      删除 cookies 文件
// @Description  删除已上传的 YouTube cookies 文件。删除后需要重新上传才能下载受限制的视频
// @Tags         cookies
// @Accept       json
// @Produce      json
// @Success      200  {object}  Response{data=CookiesStatusData}  "删除成功"
// @Failure      404  {object}  Response  "Cookies 文件不存在"
// @Failure      500  {object}  Response  "删除文件失败"
// @Router       /api/v1/cookies [delete]
func (h *CookiesHandler) deleteCookies(c *gin.Context) {
	cookiesPath := h.getCookiesPath()

	// 检查文件是否存在
	if _, err := os.Stat(cookiesPath); os.IsNotExist(err) {
		NotFound(c, "Cookies 文件不存在")
		return
	}

	// 删除文件
	if err := os.Remove(cookiesPath); err != nil {
		h.Logger.Error("删除文件失败", zap.Error(err))
		InternalServerError(c, fmt.Sprintf("删除文件失败: %v", err))
		return
	}

	h.Logger.Info("Cookies 文件已删除", zap.String("path", cookiesPath))

	SuccessWithMessage(c, CookiesStatusData{
		HasCookies: false,
	}, "Cookies 文件已删除")
}

// getCookiesPath 获取 cookies 文件路径
func (h *CookiesHandler) getCookiesPath() string {
	if h.Config.Workflow.CookiesFile != "" {
		return h.Config.Workflow.CookiesFile
	}
	// 默认路径：./cookies/ytdlp_cookies.txt
	return filepath.Join("cookies", "ytdlp_cookies.txt")
}

// validateCookiesContent 验证 cookies 文件内容格式
func (h *CookiesHandler) validateCookiesContent(content string) bool {
	lines := strings.Split(content, "\n")

	// 至少需要有一行有效内容
	validLineCount := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Netscape cookies 格式：domain flag path secure expiration name value
		// 至少需要 7 个字段
		fields := strings.Split(line, "\t")
		if len(fields) >= 7 {
			validLineCount++
		}
	}

	// 至少需要一行有效的 cookie 数据
	return validLineCount > 0
}

package server

import (
	"embed"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// embedFS 嵌入前端构建产物（out 目录）
// 注意：开发阶段此目录可能不存在，编译时会警告但不会失败
//
//go:embed all:out
var embedFS embed.FS

// isWebEmpty 检查是否嵌入了前端文件（用于判断是否为开发模式）
func isWebEmpty() bool {
	entries, err := fs.ReadDir(embedFS, ".")
	if err != nil {
		return true
	}
	return len(entries) == 0
}

// ServeStaticWeb 提供嵌入的前端静态文件服务
// 开发模式：返回 nil，由前端独立运行
// 生产模式：从 embed.FS 提供静态文件
func ServeStaticWeb(r *gin.Engine, logger *zap.Logger, isDev bool) {
	// logger.Info("🔍 ServeStaticWeb 被调用", zap.Bool("isDev", isDev))
	
	if isDev {
		logger.Info("开发模式：前端需独立运行 (npm run dev)")
		return
	}

	// 构建生产模式的静态文件服务
//	logger.Info("生产模式：加载嵌入的前端静态文件")

	// 检查是否为空（未构建前端）
	isEmpty := isWebEmpty()
	// logger.Info("🔍 检查嵌入文件", zap.Bool("isEmpty", isEmpty))
	
	if isEmpty {
		// logger.Warn("前端文件未嵌入（internal/server/out 为空），请运行 `make web-export`")
		return
	}

	// 使用嵌入的文件系统
	outFS, err := fs.Sub(embedFS, "out")
	if err != nil {
		// logger.Error("❌ 无法获取 out 子目录", zap.Error(err))
		return
	}
	// logger.Info("✅ 成功挂载 out 目录")

	// 静态文件处理函数
	serveFile := func(c *gin.Context, path string) {
		// 移除前导斜杠
		path = strings.TrimPrefix(path, "/")
		
		// 空路径默认为 index.html
		if path == "" {
			path = "index.html"
		}
		
		// logger.Info("🔍 serveFile 尝试打开", zap.String("path", path))

		// 读取文件
		file, err := outFS.Open(path)
		if err != nil {
			// logger.Warn("❌ 文件打开失败", zap.String("path", path), zap.Error(err))
			c.Status(http.StatusNotFound)
			return
		}
		defer file.Close()
		
		// logger.Info("✅ 文件打开成功", zap.String("path", path))

		// 获取文件信息
		stat, err := file.Stat()
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}

		// 如果是目录，尝试 index.html
		if stat.IsDir() {
			indexPath := filepath.Join(path, "index.html")
			file.Close() // 关闭目录
			
			file, err = outFS.Open(indexPath)
			if err != nil {
				c.Status(http.StatusNotFound)
				return
			}
			defer file.Close()
			
			stat, err = file.Stat()
			if err != nil {
				c.Status(http.StatusInternalServerError)
				return
			}
			path = indexPath
		}

		// 设置 Content-Type
		ext := filepath.Ext(path)
		contentType := getContentType(ext)
		if contentType != "" {
			c.Header("Content-Type", contentType)
		}

		// 设置缓存头（生产环境）
		if strings.HasPrefix(path, "_next/") {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			c.Header("Cache-Control", "public, max-age=0, must-revalidate")
		}

		// 复制文件内容到响应
		c.Status(http.StatusOK)
		io.Copy(c.Writer, file)
	}

	// 前端静态资源路由（_next, images, etc.）
	r.GET("/_next/*filepath", func(c *gin.Context) {
		serveFile(c, c.Request.URL.Path)
	})

	// 其他静态文件
	r.GET("/favicon.ico", func(c *gin.Context) {
		serveFile(c, "/favicon.ico")
	})

	r.GET("/robots.txt", func(c *gin.Context) {
		serveFile(c, "/robots.txt")
	})

	r.GET("/plans.json", func(c *gin.Context) {
		serveFile(c, "/plans.json")
	})

	// SPA 路由：所有非 API 请求返回对应文件或 index.html
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		//logger.Info("🔍 NoRoute 被调用", zap.String("path", path))

		// API 路由跳过（返回 404）
		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/static/") {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "route not found",
				"path":  path,
			})
			return
		}

		// 移除前导斜杠和尾随斜杠
		cleanPath := strings.TrimPrefix(strings.TrimSuffix(path, "/"), "/")
		//logger.Info("🔍 cleanPath", zap.String("cleanPath", cleanPath))
		
		// 尝试不同的路径策略
		tryPaths := []string{
			cleanPath,                           // 原路径
			cleanPath + ".html",                 // 添加 .html
			cleanPath + "/index.html",           // 目录的 index.html
			filepath.Join(cleanPath, "index.html"), // 使用 filepath.Join
		}

		// 尝试找到存在的文件
		found := false
		for _, tryPath := range tryPaths {
			if tryPath == "" {
				continue
			}
			// logger.Info("🔍 尝试路径", zap.String("tryPath", tryPath))
			if _, err := fs.Stat(outFS, tryPath); err == nil {
				// logger.Info("✅ 找到文件", zap.String("path", tryPath))
				serveFile(c, "/"+tryPath)
				found = true
				break
			}
		}

		// 如果都找不到，返回 index.html（SPA 路由）
		if !found {
			// logger.Info("🔍 返回默认 index.html")
			serveFile(c, "/index.html")
		}
	})

	//logger.Info("✅ 前端静态文件服务已启动")
}

// getContentType 根据文件扩展名返回 Content-Type
func getContentType(ext string) string {
	switch ext {
	case ".html":
		return "text/html; charset=utf-8"
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	// 视频 / 音频
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".ogg":
		return "video/ogg"
	case ".mp3":
		return "audio/mpeg"
	case ".m4a":
		return "audio/mp4"
	// 字幕
	case ".srt":
		return "text/plain; charset=utf-8"
	case ".vtt":
		return "text/vtt; charset=utf-8"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	case ".eot":
		return "application/vnd.ms-fontobject"
	default:
		return ""
	}
}


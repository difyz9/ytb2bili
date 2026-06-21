package analytics

import (
	"time"

	"github.com/gin-gonic/gin"
)

// Middleware 创建 Analytics 中间件
func (c *Client) Middleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if c.client == nil {
			ctx.Next()
			return
		}

		start := time.Now()
		
		// 处理请求
		ctx.Next()
		
		// 记录请求事件
		duration := time.Since(start)
		
		c.Track("http_request", map[string]interface{}{
			"method":       ctx.Request.Method,
			"path":         ctx.Request.URL.Path,
			"status_code":  ctx.Writer.Status(),
			"duration_ms":  duration.Milliseconds(),
			"user_agent":   ctx.Request.UserAgent(),
			"ip":           ctx.ClientIP(),
			"query":        ctx.Request.URL.RawQuery,
		})
	}
}

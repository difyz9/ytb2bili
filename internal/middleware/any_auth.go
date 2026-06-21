package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// AnyAuthMiddleware is a unified auth middleware.
// It checks if an upstream middleware has already set uid on the gin context.
// If not, it returns 401.
func AnyAuthMiddleware(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// ── Upstream middleware already set uid ────────────────────────────
		if uid, exists := c.Get("uid"); exists && uid != nil && uid != "" {
			c.Next()
			return
		}

		// ── Local JWT bearer auth fallback ─────────────────────────────────
		token := ParseBearerToken(c.GetHeader("Authorization"))
		if token != "" && strings.TrimSpace(jwtSecret) != "" {
			if claims, err := ParseLocalToken(token, jwtSecret, localAccessTokenType); err == nil {
				c.Set("uid", claims.UID)
				if strings.TrimSpace(claims.Email) != "" {
					c.Set("email", claims.Email)
				}
				c.Set("provider", "email")
				c.Next()
				return
			}
		}

		// ── Not authenticated → 401 ────────────────────────────────────────
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未授权，请先登录"})
	}
}

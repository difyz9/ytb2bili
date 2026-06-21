package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zolagz/ytb2bili/internal/service"
)

const agentOpenPrincipalContextKey = "agentOpenPrincipal"

func APIKeyAuthMiddleware(agentOpen *service.AgentOpenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if agentOpen == nil || !agentOpen.Enabled() {
			abortAgentOpen(c, http.StatusServiceUnavailable, "service_unavailable")
			return
		}

		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		if !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			abortAgentOpen(c, http.StatusUnauthorized, "invalid_api_key")
			return
		}

		rawKey := strings.TrimSpace(authHeader[len("Bearer "):])
		principal, err := agentOpen.AuthenticateAPIKey(c.Request.Context(), rawKey)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrAgentOpenDisabled):
				abortAgentOpen(c, http.StatusServiceUnavailable, "service_unavailable")
			case errors.Is(err, service.ErrInvalidAPIKey):
				abortAgentOpen(c, http.StatusUnauthorized, "invalid_api_key")
			default:
				abortAgentOpen(c, http.StatusInternalServerError, "internal_error")
			}
			return
		}

		c.Set(agentOpenPrincipalContextKey, principal)
		c.Next()
	}
}

func RequireAgentScopes(scopes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal := GetAgentOpenPrincipal(c)
		if principal == nil || !principal.HasScopes(scopes...) {
			abortAgentOpen(c, http.StatusForbidden, "insufficient_scope")
			return
		}
		c.Next()
	}
}

func GetAgentOpenPrincipal(c *gin.Context) *service.AgentOpenPrincipal {
	value, ok := c.Get(agentOpenPrincipalContextKey)
	if !ok {
		return nil
	}
	principal, _ := value.(*service.AgentOpenPrincipal)
	return principal
}

func abortAgentOpen(c *gin.Context, code int, message string) {
	c.AbortWithStatusJSON(code, gin.H{
		"code":    code,
		"data":    gin.H{},
		"message": message,
	})
}
// Package middleware provides HTTP middleware for the Gin engine.
package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS returns a Gin middleware that allows cross-origin requests with
// credentials.  The allowed origin is dynamically reflected from the request
// so that any front-end origin works without enumerating hosts.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			allowHeaders := []string{
				"Origin",
				"Content-Type",
				"Accept",
				"Authorization",
				"X-App-Id",
				"X-Timestamp",
				"X-Nonce",
				"X-Sign",
				"X-Project-Id",
			}
			if requestedHeaders := strings.TrimSpace(c.Request.Header.Get("Access-Control-Request-Headers")); requestedHeaders != "" {
				allowHeaders = append(allowHeaders, requestedHeaders)
			}

			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Vary", "Origin, Access-Control-Request-Method, Access-Control-Request-Headers")
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Writer.Header().Set("Access-Control-Allow-Headers", strings.Join(allowHeaders, ", "))
			c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Length")
			c.Writer.Header().Set("Access-Control-Max-Age", "43200") // 12 hours
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

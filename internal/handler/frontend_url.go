package handler

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// DefaultFrontendBaseURL is used only when no forwarded/request host information is available.
const DefaultFrontendBaseURL = "http://localhost:8096"

func resolveFrontendBaseURL(request *http.Request) string {
	if configured := strings.TrimRight(strings.TrimSpace(os.Getenv("FRONTEND_BASE_URL")), "/"); configured != "" {
		return configured
	}

	if request != nil {
		if origin := strings.TrimRight(strings.TrimSpace(request.Header.Get("Origin")), "/"); origin != "" {
			return origin
		}

		proto := strings.TrimSpace(request.Header.Get("X-Forwarded-Proto"))
		if proto == "" {
			if request.TLS != nil {
				proto = "https"
			} else {
				proto = "http"
			}
		}

		host := strings.TrimSpace(request.Header.Get("X-Forwarded-Host"))
		if host == "" {
			host = strings.TrimSpace(request.Host)
		}

		if host != "" {
			return proto + "://" + host
		}
	}

	return DefaultFrontendBaseURL
}

func frontendURL(c *gin.Context, path string) string {
	return resolveFrontendBaseURL(c.Request) + path
}
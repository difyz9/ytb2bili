package handler

import "github.com/gin-gonic/gin"

// RouteRegistrar is implemented by every handler that exposes HTTP routes.
// Handlers tagged with group:"routes" are auto-registered on startup by Module.
type RouteRegistrar interface {
	RegisterRoutes(r *gin.Engine)
}

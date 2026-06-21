package background

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
)

// Module owns background jobs that do not expose HTTP routes.
//
// CronJob already manages its own fx.Lifecycle hooks; the purpose of this
// module is to keep background task ownership independent from HTTP route
// wiring.
var Module = fx.Module("background",
	fx.Provide(NewCronJob),
	fx.Invoke(registerRoutes),
)

func registerRoutes(r *gin.Engine, job *CronJob) {
	if r == nil || job == nil {
		return
	}
	r.GET("/api/v1/background/status", job.StatusHandler)
}
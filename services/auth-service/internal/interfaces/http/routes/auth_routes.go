package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"auth-service/pkg/metrics"
)

// SetupAuthRoutes serves only /health (Compose healthcheck) and /metrics
// (Prometheus). The auth API is gRPC-only, reached through the gateway.
func SetupAuthRoutes(router *gin.Engine) {
	router.Use(metrics.GinMiddleware("auth-service"))
	router.GET("/metrics", gin.WrapH(metrics.Handler()))
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"service": "auth-service", "status": "running"})
	})
}

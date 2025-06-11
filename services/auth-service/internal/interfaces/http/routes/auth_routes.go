// auth-service/internal/interfaces/http/routes/auth_routes.go
package routes

import (
	"github.com/gin-gonic/gin"

	"auth-service/internal/application/services"
	"auth-service/internal/interfaces/http/handlers"
	"auth-service/internal/interfaces/http/middleware"
	"auth-service/pkg/logger"
)

func SetupAuthRoutes(router *gin.Engine, authService *services.AuthService, logger *logger.Logger) {
	// Initialize handlers
	authHandler := handlers.NewAuthHandler(authService, logger)

	// Add global middleware
	router.Use(middleware.ErrorHandler(logger))
	router.Use(middleware.RequestLogger(logger))
	router.Use(middleware.CORS())

	// Health check
	router.GET("/health", authHandler.HealthCheck)

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		auth := v1.Group("/auth")
		{
			// Google OAuth flow
			auth.GET("/google", authHandler.GetGoogleAuthURL)
			auth.GET("/google/callback", authHandler.GoogleCallback)
			auth.POST("/exchange", authHandler.ExchangeAuthCode)
			
			// Your existing endpoints
			auth.POST("/google", authHandler.GoogleLogin) // Keep if still needed
			auth.POST("/refresh", authHandler.RefreshToken)
			auth.POST("/logout", authHandler.Logout)
			auth.GET("/validate", authHandler.ValidateToken)
		}
	}
}
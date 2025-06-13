package routes

import (
	"github.com/gin-gonic/gin"

	"auth-service/internal/application/services"
	"auth-service/internal/interfaces/http/handlers"
	"auth-service/internal/interfaces/http/middleware"
	"auth-service/pkg/logger"
)

// Fix 4: Update internal/interfaces/http/routes/auth_routes.go
// Clean up routes to remove legacy endpoint

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
			// Modern OAuth2 flow (recommended)
			auth.GET("/google", authHandler.GetGoogleAuthURL)          // Step 1: Get auth URL
			auth.GET("/google/callback", authHandler.GoogleCallback)  // Step 2: Handle callback
			auth.POST("/exchange", authHandler.ExchangeAuthCode)       // Step 3: Exchange for tokens
			
			// Token management
			auth.POST("/refresh", authHandler.RefreshToken)
			auth.POST("/logout", authHandler.Logout)
			auth.GET("/validate", authHandler.ValidateToken)
		}
	}
}
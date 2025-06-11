package routes

import (
	"github.com/gin-gonic/gin"

	"api-gateway/internal/clients"
	"api-gateway/internal/config"
	"api-gateway/internal/handlers"
	"api-gateway/internal/middleware"
)

func SetupRoutes(
	router *gin.Engine,
	authHandler *handlers.AuthHandler,
	userHandler *handlers.UserHandler,
	healthHandler *handlers.HealthHandler,
	authClient *clients.AuthClient,
	redisClient *clients.RedisClient,
	cfg *config.Config,
) {
	// Health check route (no auth required)
	router.GET("/health", healthHandler.HealthCheck)

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Auth routes (no authentication required)
		authGroup := v1.Group("/auth")
		{
			// Modern OAuth2 flow (recommended)
			authGroup.GET("/google", authHandler.GetGoogleAuthURL)
			authGroup.GET("/google/callback", authHandler.GoogleCallback)
			authGroup.POST("/exchange", authHandler.ExchangeAuthCode)
			
			// Legacy endpoint (keep for backward compatibility)
			authGroup.POST("/google", authHandler.GoogleLogin)
			
			// Token management
			authGroup.POST("/refresh", authHandler.RefreshToken)
			
			// Protected auth routes
			authProtected := authGroup.Group("")
			authProtected.Use(middleware.AuthMiddleware(authClient))
			{
				authProtected.POST("/logout", authHandler.Logout)
				authProtected.GET("/validate", authHandler.ValidateToken)
			}
		}

		// Public routes (no authentication required)
		publicGroup := v1.Group("/public")
		publicGroup.Use(middleware.OptionalAuthMiddleware(authClient))
		{
			// Public user routes
			publicUsers := publicGroup.Group("/users")
			{
				publicUsers.GET("/search", userHandler.SearchUsers)
				publicUsers.GET("/stats", userHandler.GetStats)
				publicUsers.GET("/:id/profile", userHandler.GetUserProfile)
			}
		}

		// Protected routes (authentication required)
		protectedGroup := v1.Group("")
		protectedGroup.Use(middleware.AuthMiddleware(authClient))
		{
			// User routes
			users := protectedGroup.Group("/users")
			{
				users.POST("", userHandler.CreateUser)
				users.GET("", userHandler.ListUsers)
				users.GET("/:id", userHandler.GetUser)
				users.PUT("/:id", userHandler.UpdateUser)
				users.DELETE("/:id", userHandler.DeleteUser)
			}
		}
	}
}
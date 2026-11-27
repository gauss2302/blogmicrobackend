package routes

import (
	"github.com/gin-gonic/gin"

	"user-service/internal/application/services"
	"user-service/internal/interfaces/http/handlers"
	"user-service/internal/interfaces/http/middleware"
	"user-service/pkg/auth"
	"user-service/pkg/logger"
)

// SetupUserRoutes wires the HTTP API. Protected routes authenticate the caller
// from a signed access token (validator); trustMode gates the insecure_dev
// X-User-ID fallback and is never permissive in production.
func SetupUserRoutes(router *gin.Engine, userService *services.UserService, validator *auth.Validator, revocations auth.RevocationChecker, trustMode string, logger *logger.Logger) {
	// Initialize handlers
	userHandler := handlers.NewUserHandler(userService, logger)

	// Add global middleware
	router.Use(middleware.ErrorHandler(logger))
	router.Use(middleware.RequestLogger(logger))
	router.Use(middleware.CORS())

	// Health check (no auth required)
	router.GET("/health", userHandler.HealthCheck)

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		users := v1.Group("/users")
		{
			// Public routes (no auth required)
			users.GET("/search", userHandler.SearchUsers)
			users.GET("/stats", userHandler.GetStats)
			users.GET("/:id/profile", userHandler.GetUserProfile)

			// Protected routes (auth required)
			protected := users.Group("")
			protected.Use(middleware.AuthMiddleware(validator, revocations, trustMode, logger))
			{
				protected.GET("", userHandler.ListUsers)
				protected.GET("/:id", userHandler.GetUser)
				protected.PUT("/:id", userHandler.UpdateUser)
				protected.DELETE("/:id", userHandler.DeleteUser)
			}
		}
	}
}

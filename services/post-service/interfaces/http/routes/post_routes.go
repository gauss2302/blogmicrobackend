package routes

import (
	"github.com/gin-gonic/gin"

	"post-service/interfaces/http/handlers"
	"post-service/interfaces/http/middleware"
	"post-service/internal/application/services"

	"post-service/pkg/logger"
)

func SetupPostRoutes(router *gin.Engine, postService *services.PostService, logger *logger.Logger) {
	// Initialize handlers
	postHandler := handlers.NewPostHandler(postService, logger)

	// Add global middleware
	router.Use(middleware.ErrorHandler(logger))
	router.Use(middleware.RequestLogger(logger))
	router.Use(middleware.CORS())

	// Health check (no auth required)
	router.GET("/health", postHandler.HealthCheck)

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		posts := v1.Group("/posts")
		{
			// Public routes (no auth required)
			posts.GET("", postHandler.ListPosts)                    // List published posts
			posts.GET("/search", postHandler.SearchPosts)           // Search published posts
			posts.GET("/stats", postHandler.GetStats)               // Public post statistics
			posts.GET("/slug/:slug", postHandler.GetPostBySlug)     // Get post by slug (published only)
			posts.GET("/user/:userId", postHandler.GetUserPosts)    // Get user's published posts

			// Protected routes (auth required)
			protected := posts.Group("")
			protected.Use(middleware.AuthMiddleware())
			{
				protected.POST("", postHandler.CreatePost)          // Create new post
				protected.GET("/:id", postHandler.GetPost)          // Get post by ID (own posts or published)
				protected.PUT("/:id", postHandler.UpdatePost)       // Update own post
				protected.DELETE("/:id", postHandler.DeletePost)    // Delete own post
			}
		}
	}
}
package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"api-gateway/internal/clients"
	"api-gateway/internal/config"
	"api-gateway/internal/handlers"
	"api-gateway/internal/middleware"
	"api-gateway/pkg/metrics"
	"api-gateway/pkg/utils"
)

func SetupRoutes(
	router *gin.Engine,
	authHandler *handlers.AuthHandler,
	userHandler *handlers.UserHandler,
	postHandler *handlers.PostHandler,
	searchHandler *handlers.SearchHandler,
	healthHandler *handlers.HealthHandler,
	authClient *clients.AuthClient,
	redisClient *clients.RedisClient,
	cfg *config.Config,
) {
	// Health check route (no auth required)
	router.GET("/health", healthHandler.HealthCheck)
	router.GET("/metrics", gin.WrapH(metrics.Handler()))

	// Root index — a plain GET / would otherwise hit Gin's bare-text 404.
	router.GET("/", func(c *gin.Context) {
		utils.SuccessResponse(c, http.StatusOK, "api-gateway", gin.H{
			"service": "api-gateway",
			"status":  "ok",
			"endpoints": []string{
				"/health",
				"/metrics",
				"/api/v1/auth",
				"/api/v1/public/users",
				"/api/v1/public/posts",
				"/api/v1/users",
				"/api/v1/posts",
				"/api/v1/search",
			},
		})
	})

	// Consistent JSON for unmatched routes instead of Gin's plain "404 page not found".
	router.NoRoute(func(c *gin.Context) {
		utils.ErrorResponse(c, http.StatusNotFound, "NOT_FOUND", "Route not found")
	})

	// Global middleware
	router.Use(middleware.RequestValidator(cfg.RequestMaxBodyBytes))
	router.Use(middleware.RateLimit(redisClient, cfg.RateLimit))

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Auth routes (no authentication required)
		authGroup := v1.Group("/auth")
		{
			// OAuth2 redirect endpoints (general limiter only).
			authGroup.GET("/google", authHandler.GetGoogleAuthURL)
			authGroup.GET("/google/callback", authHandler.GoogleCallback)

			// Credential/token endpoints carry a stricter per-IP limit to blunt
			// brute-force, credential stuffing, and auth_code/refresh-token guessing.
			authLimited := authGroup.Group("")
			authLimited.Use(middleware.AuthRateLimit(redisClient, cfg.RateLimit))
			{
				// Email/password
				authLimited.POST("/register", authHandler.Register)
				authLimited.POST("/login", authHandler.Login)

				// OAuth2 code exchange
				authLimited.POST("/exchange", authHandler.ExchangeAuthCode)

				// Token management
				authLimited.POST("/refresh", authHandler.RefreshToken)
			}

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

			// Public post routes
			publicPosts := publicGroup.Group("/posts")
			{
				publicPosts.GET("", postHandler.ListPosts)
				publicPosts.GET("/search", postHandler.SearchPosts)
				// publicPosts.GET("/stats", postHandler.GetPostStats)
				publicPosts.GET("/slug/:slug", postHandler.GetPostBySlug)
				publicPosts.GET("/user/:userId", postHandler.GetUserPosts)
			}
		}

		// Protected routes (authentication required)
		protectedGroup := v1.Group("")
		protectedGroup.Use(middleware.AuthMiddleware(authClient))
		{
			// Combined search (users + posts, cursor-based)
			protectedGroup.GET("/search", searchHandler.Search)

			// User routes
			users := protectedGroup.Group("/users")
			{
				users.POST("", userHandler.CreateUser)
				users.GET("", userHandler.ListUsers)
				users.GET("/:id", userHandler.GetUser)
				users.PUT("/:id", userHandler.UpdateUser)
				users.DELETE("/:id", userHandler.DeleteUser)
				users.POST("/:id/follow", userHandler.Follow)
				users.DELETE("/:id/follow", userHandler.Unfollow)
				users.GET("/:id/followers", userHandler.GetFollowers)
				users.GET("/:id/following", userHandler.GetFollowing)
			}

			// Post routes
			posts := protectedGroup.Group("/posts")
			{
				posts.POST("", postHandler.CreatePost)
				posts.GET("/:id", postHandler.GetPost)
				posts.PUT("/:id", postHandler.UpdatePost)
				posts.DELETE("/:id", postHandler.DeletePost)
			}
		}
	}
}

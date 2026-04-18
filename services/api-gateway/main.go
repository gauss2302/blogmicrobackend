package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"api-gateway/internal/clients"
	"api-gateway/internal/config"
	"api-gateway/internal/handlers"
	"api-gateway/internal/middleware"
	"api-gateway/internal/routes"
	"api-gateway/pkg/logger"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	appLogger := logger.New(cfg.LogLevel)

	// Initialize service clients
	redisClient := clients.NewRedisClient(cfg.Redis)
	authClient, err := clients.NewAuthClient(cfg.Services.AuthGRPCAddr, cfg.GRPCTLS, appLogger)
	if err != nil {
		appLogger.Fatal("Failed to connect to auth service: " + err.Error())
	}
	userClient, err := clients.NewUserClient(cfg.Services.UserGRPCAddr, cfg.GRPCTLS, appLogger)
	if err != nil {
		appLogger.Fatal("Failed to connect to user service: " + err.Error())
	}
	postClient, err := clients.NewPostClient(cfg.Services.PostGRPCAddr, cfg.GRPCTLS, appLogger)
	if err != nil {
		appLogger.Fatal("Failed to connect to post service: " + err.Error())
	}
	searchClient, err := clients.NewSearchClient(cfg.Services.SearchGRPCAddr, cfg.GRPCTLS, appLogger)
	if err != nil {
		appLogger.Fatal("Failed to connect to search service: " + err.Error())
	}

	// Test service connections
	if err := testServiceConnections(authClient, userClient, postClient, searchClient, appLogger); err != nil {
		appLogger.Warn("Some services are not available: " + err.Error())
	}

	authHandler := handlers.NewAuthHandler(authClient, cfg, appLogger)
	userHandler := handlers.NewUserHandler(userClient, appLogger)
	postHandler := handlers.NewPostHandler(postClient, appLogger)
	searchHandler := handlers.NewSearchHandler(searchClient, appLogger)
	healthHandler := handlers.NewHealthHandler(authClient, userClient, postClient, cfg.Services.NotificationURL, appLogger)

	// Setup HTTP server
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Global middleware
	router.Use(gin.Recovery())
	router.Use(middleware.RequestLogger(appLogger))
	router.Use(middleware.CORS(cfg.CORS))
	router.Use(middleware.SecurityHeaders(cfg.Environment))

	// Setup routes
	routes.SetupRoutes(router, authHandler, userHandler, postHandler, searchHandler, healthHandler, authClient, redisClient, cfg)

	// Create HTTP server
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeout) * time.Second,
	}

	// Start server in goroutine
	go func() {
		appLogger.Info("API Gateway starting on port " + cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.Fatal("Failed to start server: " + err.Error())
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	appLogger.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		appLogger.Fatal("Server forced to shutdown: " + err.Error())
	}

	// Close service clients
	err = redisClient.Close()
	if err != nil {
		return
	}
	if err := authClient.Close(); err != nil {
		appLogger.Warn("Failed to close auth client: " + err.Error())
	}
	if err := userClient.Close(); err != nil {
		appLogger.Warn("Failed to close user client: " + err.Error())
	}
	if err := postClient.Close(); err != nil {
		appLogger.Warn("Failed to close post client: " + err.Error())
	}
	if err := searchClient.Close(); err != nil {
		appLogger.Warn("Failed to close search client: " + err.Error())
	}

	appLogger.Info("Server exited")
}

func testServiceConnections(authClient *clients.AuthClient, userClient *clients.UserClient, postClient *clients.PostClient, searchClient *clients.SearchClient, logger *logger.Logger) error {

	logger.Info("Testing service connections...")

	// Test auth service
	if err := authClient.HealthCheck(context.Background()); err != nil {
		logger.Warn("Auth service health check failed: " + err.Error())
	} else {
		logger.Info("Auth service connected successfully")
	}

	// Test user service
	if err := userClient.HealthCheck(context.Background()); err != nil {
		logger.Warn("User service health check failed: " + err.Error())
	} else {
		logger.Info("User service connected successfully")
	}

	// Test post service
	if err := postClient.HealthCheck(context.Background()); err != nil {
		logger.Warn("Post service health check failed: " + err.Error())
	} else {
		logger.Info("Post service connected successfully")
	}

	// Test search service
	if err := searchClient.HealthCheck(context.Background()); err != nil {
		logger.Warn("Search service health check failed: " + err.Error())
	} else {
		logger.Info("Search service connected successfully")
	}

	return nil
}

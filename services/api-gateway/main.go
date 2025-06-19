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
	authClient := clients.NewAuthClient(cfg.Services.AuthURL, appLogger)
	userClient := clients.NewUserClient(cfg.Services.UserURL, appLogger)
	postClient := clients.NewPostClient(cfg.Services.PostURL, appLogger)

	// Test service connections
	if err := testServiceConnections(authClient, userClient, postClient, appLogger); err != nil {
		appLogger.Warn("Some services are not available: " + err.Error())
	}

	authHandler := handlers.NewAuthHandler(authClient, userClient, appLogger)
	userHandler := handlers.NewUserHandler(userClient, appLogger)
	postHandler := handlers.NewPostHandler(postClient, appLogger)
	healthHandler := handlers.NewHealthHandler(authClient, userClient, postClient, appLogger)

	// Setup HTTP server
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Global middleware
	router.Use(gin.Recovery())
	router.Use(middleware.RequestLogger(appLogger))
	router.Use(middleware.CORS())

	// Setup routes
	routes.SetupRoutes(router, authHandler, userHandler, postHandler, healthHandler, authClient, redisClient, cfg)

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
	authClient.Close()
	userClient.Close()
	postClient.Close()

	appLogger.Info("Server exited")
}

func testServiceConnections(authClient *clients.AuthClient, userClient *clients.UserClient, postClient *clients.PostClient, logger *logger.Logger) error {

	logger.Info("Testing service connections...")

	// Test auth service
	if err := authClient.HealthCheck(); err != nil {
		logger.Warn("Auth service health check failed: " + err.Error())
	} else {
		logger.Info("Auth service connected successfully")
	}

	// Test user service
	if err := userClient.HealthCheck(); err != nil {
		logger.Warn("User service health check failed: " + err.Error())
	} else {
		logger.Info("User service connected successfully")
	}

	// Test post service
	if err := postClient.HealthCheck(); err != nil {
		logger.Warn("Post service health check failed: " + err.Error())
	} else {
		logger.Info("Post service connected successfully")
	}

	return nil
}

package main

import (
	"auth-service/internal/application/services"
	"auth-service/internal/config"
	"auth-service/internal/infrastructure/oauth"
	"auth-service/internal/infrastructure/redis"
	"auth-service/internal/interfaces/http/routes"
	"auth-service/pkg/logger"
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	appLogger := logger.New(cfg.LogLevel)

	// Initialize dependencies
	tokenRepo := redis.NewTokenRepository(cfg.Redis)
	googleProvider := oauth.NewGoogleProvider(cfg.Google)
	authService := services.NewAuthService(tokenRepo, googleProvider, cfg.JWT, appLogger)

	// Setup HTTP server
	router := gin.New()
	router.Use(gin.Recovery())

	// Setup routes
	routes.SetupAuthRoutes(router, authService, appLogger)

	// Create HTTP server
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		appLogger.Info("Auth service starting on port " + cfg.Port)
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

	appLogger.Info("Server exited")
}

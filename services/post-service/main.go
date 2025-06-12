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

	"post-service/interfaces/http/routes"
	"post-service/internal/application/services"
	"post-service/internal/config"
	"post-service/internal/infrastructure/postgres"

	"post-service/pkg/logger"
)

func main() {

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}


	appLogger := logger.New(cfg.LogLevel)


	db, err := postgres.NewConnection(cfg.Database)
	if err != nil {
		appLogger.Fatal("Failed to connect to database: " + err.Error())
	}
	defer db.Close()


	if err := postgres.RunMigrations(db); err != nil {
		appLogger.Fatal("Failed to run migrations: " + err.Error())
	}


	postRepo := postgres.NewPostRepository(db)


	postService := services.NewPostService(postRepo, appLogger)


	if cfg.Port == "8083" && os.Getenv("ENVIRONMENT") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	
	router := gin.New()
	router.Use(gin.Recovery())


	routes.SetupPostRoutes(router, postService, appLogger)


	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}


	go func() {
		appLogger.Info("Post service starting on port " + cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.Fatal("Failed to start server: " + err.Error())
		}
	}()


	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	appLogger.Info("Shutting down server...")


	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		appLogger.Fatal("Server forced to shutdown: " + err.Error())
	}

	appLogger.Info("Server exited")
}
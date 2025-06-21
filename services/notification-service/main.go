package main

import (
	"context"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"log"
	"net/http"
	"notification-service/internal/application/services"
	"notification-service/internal/config"
	postgres "notification-service/internal/infrastructure"
	"notification-service/internal/infrastructure/rabbitmq"
	"notification-service/internal/interface/routes"
	"notification-service/pkg/logger"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	appLogger := logger.New(cfg.LogLevel)

	db, err := postgres.NewConntection(cfg.Database)
	if err != nil {
		appLogger.Fatalf("failed to connect to db: %v", err)
	}
	defer db.Close()

	if err := postgres.RunMigrations(db); err != nil {
		appLogger.Fatal("failed to run migrations: " + err.Error())
	}

	notificationRepo := postgres.NewNotificationRepository(db)
	notificationService := services.NewNotificationService(notificationRepo, appLogger)
	rabbitMQClient := rabbitmq.NewClient(cfg.RabbitMQ, appLogger)

	if err := rabbitMQClient.Connect(); err != nil {
		appLogger.Fatal("failed to connect to rabbit " + err.Error())
	}
	defer rabbitMQClient.Close()

	messageHanlder := func(body []byte) error {
		return notificationService.ProcessPostCreatedEvent(context.Background(), body)
	}

	if err := rabbitMQClient.StartConsuming(messageHanlder); err != nil {
		appLogger.Fatal("failed to start consuming messages " + err.Error())
	}

	if cfg.Port == "8084" && os.Getenv("ENVIRONMENT") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())

	routes.SetupNotificationRoutes(router, notificationService, appLogger)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := notificationService.CleanupOldNotifications(context.Background(), 30); err != nil {
					appLogger.Error("failed to cleanup old notifs: " + err.Error())
				}
			}
		}
	}()

	go func() {
		appLogger.Info("notif server starting on port " + cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.Fatal("failed to start server " + err.Error())
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	appLogger.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		appLogger.Fatal("server forced to shutdown: " + err.Error())
	}

	appLogger.Info("server exited")
}

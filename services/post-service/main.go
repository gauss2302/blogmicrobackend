package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"post-service/internal/infrastructure/messaging"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"post-service/interfaces/http/routes"
	"post-service/internal/application/services"
	"post-service/internal/config"
	"post-service/internal/infrastructure/postgres"
	grpcinterface "post-service/internal/interfaces/grpc"

	"post-service/pkg/logger"

	postv1 "github.com/nikitashilov/microblog_grpc/proto/post/v1"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	grpc_reflection "google.golang.org/grpc/reflection"
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

	var eventPublisher *messaging.EventPublisher

	if cfg.RabbitMQ.Enabled {
		eventPublisher, err = messaging.NewEventPublisher(cfg.RabbitMQ.URL, cfg.RabbitMQ.ExchangeName, appLogger)
		if err != nil {
			appLogger.Warn("Failed to initialize event publisher, continuing without events: " + err.Error())
			eventPublisher = nil
		} else {
			appLogger.Info("Event publisher initialized successfully")
			// Ensure we close the event publisher on shutdown
			defer func() {
				if eventPublisher != nil {
					eventPublisher.Close()
				}
			}()
		}
	} else {
		appLogger.Info("RabbitMQ not configured, running without event publishing")
	}

	postService := services.NewPostService(postRepo, eventPublisher, appLogger)

	// Setup gRPC server with options
	grpcServer := grpc.NewServer(
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     15 * time.Minute,
			MaxConnectionAge:      30 * time.Minute,
			MaxConnectionAgeGrace: 5 * time.Minute,
			Time:                  5 * time.Second,
			Timeout:               1 * time.Second,
		}),
		grpc.UnaryInterceptor(unaryServerLoggingInterceptor(appLogger)),
	)
	postv1.RegisterPostServiceServer(grpcServer, grpcinterface.NewPostServer(postService, appLogger))
	grpc_reflection.Register(grpcServer)

	grpcListener, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		appLogger.Fatal("Failed to create gRPC listener: " + err.Error())
	}

	go func() {
		appLogger.Info("Post gRPC service starting on port " + cfg.GRPCPort)
		if serveErr := grpcServer.Serve(grpcListener); serveErr != nil && !errors.Is(serveErr, grpc.ErrServerStopped) {
			appLogger.Fatal("Failed to start gRPC server: " + serveErr.Error())
		}
	}()

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

	if eventPublisher != nil {
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					if !eventPublisher.IsConnected() {
						appLogger.Warn("Event publisher disconnected, attempting reconnection...")
						if err := eventPublisher.Reconnect(cfg.RabbitMQ.URL); err != nil {
							appLogger.Error("Failed to reconnect event publisher: " + err.Error())
						}
					}
				}
			}
		}()
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

	grpcServer.GracefulStop()

	if err := server.Shutdown(ctx); err != nil {
		appLogger.Fatal("HTTP server forced to shutdown: " + err.Error())
	}

	appLogger.Info("Servers exited")
}

// unaryServerLoggingInterceptor logs gRPC server requests and responses
func unaryServerLoggingInterceptor(logger *logger.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)

		if err != nil {
			logger.Warn(fmt.Sprintf("gRPC method %s failed: %v (duration: %v)", info.FullMethod, err, duration))
		} else {
			logger.Debug(fmt.Sprintf("gRPC method %s succeeded (duration: %v)", info.FullMethod, duration))
		}

		return resp, err
	}
}

// main.go
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
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"user-service/internal/application/services"
	"user-service/internal/config"
	"user-service/internal/infrastructure/postgres"
	grpcinterface "user-service/internal/interfaces/grpc"
	"user-service/internal/interfaces/http/routes"
	"user-service/pkg/logger"

	userv1 "github.com/nikitashilov/microblog_grpc/proto/user/v1"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	grpc_reflection "google.golang.org/grpc/reflection"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	appLogger := logger.New(cfg.LogLevel)

	// Initialize database connection
	db, err := postgres.NewConnection(cfg.Database)
	if err != nil {
		appLogger.Fatal("Failed to connect to database: " + err.Error())
	}
	defer db.Close()

	// Run migrations
	if err := postgres.RunMigrations(db); err != nil {
		appLogger.Fatal("Failed to run migrations: " + err.Error())
	}

	// Initialize repositories
	userRepo := postgres.NewUserRepository(db)
	followRepo := postgres.NewFollowRepository(db)

	// Initialize services
	userService := services.NewUserService(userRepo, followRepo, appLogger)

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
	userv1.RegisterUserServiceServer(grpcServer, grpcinterface.NewUserServer(userService, appLogger))
	grpc_reflection.Register(grpcServer)

	grpcListener, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		appLogger.Fatal("Failed to create gRPC listener: " + err.Error())
	}

	go func() {
		appLogger.Info("User gRPC service starting on port " + cfg.GRPCPort)
		if serveErr := grpcServer.Serve(grpcListener); serveErr != nil && !errors.Is(serveErr, grpc.ErrServerStopped) {
			appLogger.Fatal("Failed to start gRPC server: " + serveErr.Error())
		}
	}()

	// Setup HTTP server
	router := gin.New()
	router.Use(gin.Recovery())

	// Setup routes
	routes.SetupUserRoutes(router, userService, appLogger)

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
		appLogger.Info("User service starting on port " + cfg.Port)
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

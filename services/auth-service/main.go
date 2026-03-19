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

	"auth-service/internal/application/services"
	"auth-service/internal/clients"
	"auth-service/internal/config"
	"auth-service/internal/infrastructure/oauth"
	"auth-service/internal/infrastructure/redis"
	grpcinterface "auth-service/internal/interfaces/grpc"
	"auth-service/internal/interfaces/http/routes"
	"auth-service/pkg/logger"

	"github.com/gin-gonic/gin"
	authv1 "github.com/nikitashilov/microblog_grpc/proto/auth/v1"
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

	// Initialize dependencies
	tokenRepo := redis.NewTokenRepository(cfg.Redis)
	googleProvider := oauth.NewGoogleProvider(cfg.Google)
	userClient, err := clients.NewUserClient(cfg.Services.UserGRPCAddr)
	if err != nil {
		log.Fatalf("Failed to create user gRPC client: %v", err)
	}
	defer userClient.Close()

	authService := services.NewAuthService(tokenRepo, googleProvider, userClientAdapter{userClient}, cfg.JWT, cfg.Google, appLogger)

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
	authv1.RegisterAuthServiceServer(grpcServer, grpcinterface.NewAuthServer(authService, appLogger))
	grpc_reflection.Register(grpcServer)

	grpcListener, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		appLogger.Fatal("Failed to create gRPC listener: " + err.Error())
	}

	go func() {
		appLogger.Info("Auth gRPC service starting on port " + cfg.GRPCPort)
		if serveErr := grpcServer.Serve(grpcListener); serveErr != nil && !errors.Is(serveErr, grpc.ErrServerStopped) {
			appLogger.Fatal("Failed to start gRPC server: " + serveErr.Error())
		}
	}()

	// Setup HTTP server
	router := gin.New()
	router.Use(gin.Recovery())

	// Setup routes
	routes.SetupAuthRoutes(router, authService, appLogger)

	// Create HTTP server
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeout) * time.Second,
	}

	// Start HTTP server in goroutine
	go func() {
		appLogger.Info("Auth HTTP service starting on port " + cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.Fatal("Failed to start HTTP server: " + err.Error())
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

// userClientAdapter adapts *clients.UserClient to services.UserServiceClient (return type UserInfoResult).
type userClientAdapter struct{ *clients.UserClient }

func (a userClientAdapter) CreateUser(ctx context.Context, id, email, name, picture, password string) (services.UserInfoResult, error) {
	return a.UserClient.CreateUser(ctx, id, email, name, picture, password)
}

func (a userClientAdapter) GetUserByEmail(ctx context.Context, email string) (services.UserInfoResult, error) {
	return a.UserClient.GetUserByEmail(ctx, email)
}

func (a userClientAdapter) ValidateCredentials(ctx context.Context, email, password string) (services.UserInfoResult, error) {
	return a.UserClient.ValidateCredentials(ctx, email, password)
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

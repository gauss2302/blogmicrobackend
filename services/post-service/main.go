package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
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
	redisinfra "post-service/internal/infrastructure/redis"
	"post-service/internal/infrastructure/search"
	grpcinterface "post-service/internal/interfaces/grpc"

	"post-service/pkg/auth"
	"post-service/pkg/logger"
	"post-service/pkg/metrics"

	postv1 "github.com/gauss2302/blogmicrobackend/proto/post/v1"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	grpc_reflection "google.golang.org/grpc/reflection"
)

func main() {

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	appLogger := logger.New(cfg.LogLevel)
	metrics.Init()

	db, err := postgres.NewConnection(cfg.Database)
	if err != nil {
		appLogger.Fatal("Failed to connect to database: " + err.Error())
	}
	defer db.Close()

	if err := postgres.RunMigrations(db, search.OutboxDDL()); err != nil {
		appLogger.Fatal("Failed to run migrations: " + err.Error())
	}

	postRepo := postgres.NewPostRepository(db)

	var eventPublisher *messaging.EventPublisher

	if cfg.RabbitMQ.Enabled {
		// Fails only on a malformed URL; an unreachable broker is retried in the background.
		eventPublisher, err = messaging.NewEventPublisher(cfg.RabbitMQ.URL, cfg.RabbitMQ.ExchangeName, appLogger)
		if err != nil {
			appLogger.Warn("Failed to initialize event publisher, continuing without events: " + err.Error())
			eventPublisher = nil
		} else {
			// Stops reconnecting and closes the connection on shutdown
			defer eventPublisher.Close()
		}
	} else {
		appLogger.Info("RabbitMQ not configured, running without event publishing")
	}

	postService := services.NewPostService(postRepo, eventPublisher, appLogger)

	// Search relay: sends the post changes the triggers queue in search_outbox
	// to Kafka. Without Kafka they stay queued; a relay started later sends them.
	relayCtx, stopRelay := context.WithCancel(context.Background())
	defer stopRelay()
	relayDone := make(chan struct{})
	var relay *search.Relay
	if cfg.Kafka.Enabled {
		relay = search.NewRelay(db, cfg.Kafka.Brokers, cfg.Kafka.TopicPosts, cfg.Kafka.SASLUsername, cfg.Kafka.SASLPassword, appLogger)
		go func() {
			defer close(relayDone)
			relay.Run(relayCtx)
		}()
		appLogger.Info("Search relay (Kafka) started for topic " + cfg.Kafka.TopicPosts)

		// Rebuild: queue every post again. Gated by env so normal restarts skip
		// it; repeating it is harmless.
		if os.Getenv("SEARCH_BACKFILL_ON_START") == "true" {
			if n, err := relay.Rebuild(relayCtx); err != nil {
				appLogger.Error("Search rebuild failed: " + err.Error())
			} else {
				appLogger.Info(fmt.Sprintf("Search rebuild: %d posts queued", n))
			}
		}
	} else {
		close(relayDone)
		appLogger.Info("KAFKA_BROKERS not set: search changes stay queued in search_outbox")
	}

	// Setup gRPC server with options
	grpcOptions := []grpc.ServerOption{
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
		grpc.ChainUnaryInterceptor(
			metrics.UnaryServerInterceptor("post-service"),
			unaryServerLoggingInterceptor(appLogger),
			// Last, next to the handler: a panic becomes codes.Internal that
			// metrics and the log above still see, not a crashed process.
			grpcinterface.RecoveryInterceptor(appLogger),
		),
	}
	if cfg.GRPCTLS.Enabled {
		transportCreds, credsErr := buildServerTransportCredentials(cfg.GRPCTLS)
		if credsErr != nil {
			appLogger.Fatal("Failed to configure gRPC TLS credentials: " + credsErr.Error())
		}
		grpcOptions = append(grpcOptions, grpc.Creds(transportCreds))
	}

	grpcServer := grpc.NewServer(grpcOptions...)
	postv1.RegisterPostServiceServer(grpcServer, grpcinterface.NewPostServer(postService, appLogger))
	if cfg.EnableGRPCReflection {
		grpc_reflection.Register(grpcServer)
	}

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

	if cfg.Port == "8083" && cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(metrics.GinMiddleware("post-service"))
	router.GET("/metrics", gin.WrapH(metrics.Handler()))

	// Verifies access tokens issued by auth-service. nil when running in
	// insecure_dev with no JWT secret, in which case the X-User-ID header
	// fallback is used instead.
	var tokenValidator *auth.Validator
	if cfg.JWTSecret != "" {
		tokenValidator = auth.NewValidator(cfg.JWTSecret)
	}
	var revocations auth.RevocationChecker
	if cfg.Redis.URL != "" {
		revocations = redisinfra.NewRevocationChecker(cfg.Redis.URL, cfg.Redis.Password, cfg.Redis.DB)
	}

	routes.SetupPostRoutes(router, postService, tokenValidator, revocations, cfg.InternalHTTPTrustMode, appLogger)

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

	// A failed HTTP shutdown must not skip the gRPC stop and the relay below.
	if err := server.Shutdown(ctx); err != nil {
		appLogger.Warn("HTTP server shutdown: " + err.Error())
	}

	// In-flight RPCs get the rest of the 30s; then open ones are cut off.
	if !grpcinterface.GracefulStop(ctx, grpcServer) {
		appLogger.Warn("gRPC graceful stop timed out; open RPCs were closed")
	}

	// An unfinished batch rolls back and is sent after the next start.
	stopRelay()
	<-relayDone
	if relay != nil {
		if err := relay.Close(); err != nil {
			appLogger.Warn("Search relay close: " + err.Error())
		}
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

func buildServerTransportCredentials(tlsCfg config.GRPCTLSConfig) (credentials.TransportCredentials, error) {
	serverCert, err := tls.LoadX509KeyPair(tlsCfg.CertFile, tlsCfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("load gRPC server certificate: %w", err)
	}

	tlsConfig := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{serverCert},
	}

	if tlsCfg.RequireClientCert {
		caPEM, caErr := os.ReadFile(tlsCfg.CAFile)
		if caErr != nil {
			return nil, fmt.Errorf("read gRPC CA file: %w", caErr)
		}

		clientCAs := x509.NewCertPool()
		if ok := clientCAs.AppendCertsFromPEM(caPEM); !ok {
			return nil, fmt.Errorf("parse gRPC client CA certificate")
		}

		tlsConfig.ClientCAs = clientCAs
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return credentials.NewTLS(tlsConfig), nil
}

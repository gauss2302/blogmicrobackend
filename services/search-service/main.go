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
	"syscall"
	"time"

	searchv1 "github.com/gauss2302/blogmicrobackend/proto/search/v1"
	"search-service/internal/application/services"
	"search-service/internal/config"
	"search-service/internal/infrastructure/kafka"
	"search-service/internal/infrastructure/opensearch"
	grpcinterface "search-service/internal/interfaces/grpc"
	"search-service/pkg/logger"
	"search-service/pkg/metrics"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	grpc_reflection "google.golang.org/grpc/reflection"
)

// indicesRetryDelay is the pause between attempts to set up the OpenSearch
// indices while OpenSearch is unreachable.
const indicesRetryDelay = 5 * time.Second

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	metrics.Init()

	appLogger := logger.New(cfg.LogLevel)

	var osClient *opensearch.Client
	if cfg.OpenSearch.Enabled {
		osClient, err = opensearch.NewClient(cfg.OpenSearch.URL, cfg.OpenSearch.Username, cfg.OpenSearch.Password, appLogger)
		if err != nil {
			appLogger.Fatal("OpenSearch: " + err.Error())
		}
	} else {
		appLogger.Info("OpenSearch not configured; search will return empty results")
	}

	searchSvc, err := services.NewSearchService(
		osClient,
		cfg.UsersIndexName,
		cfg.PostsIndexName,
		cfg.UserServiceGRPC,
		services.GRPCTLSOptions{
			Enabled:  cfg.GRPCTLS.Enabled,
			CAFile:   cfg.GRPCTLS.CAFile,
			CertFile: cfg.GRPCTLS.CertFile,
			KeyFile:  cfg.GRPCTLS.KeyFile,
		},
		appLogger,
	)
	if err != nil {
		appLogger.Fatal("SearchService: " + err.Error())
	}
	defer searchSvc.Close()

	grpcOptions := []grpc.ServerOption{
		// Same as the other gRPC servers: the gateway pings idle connections
		// every 30s, and the default policy (5 min) answers that with GOAWAY.
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.ChainUnaryInterceptor(
			metrics.UnaryServerInterceptor("search-service"),
			unaryLoggingInterceptor(appLogger),
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
	searchv1.RegisterSearchServiceServer(grpcServer, grpcinterface.NewSearchServer(searchSvc, appLogger))
	if cfg.EnableGRPCReflection {
		grpc_reflection.Register(grpcServer)
	}

	listener, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		appLogger.Fatal("gRPC listen: " + err.Error())
	}

	go func() {
		appLogger.Info("Search gRPC server listening on :" + cfg.GRPCPort)
		if err := grpcServer.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			appLogger.Fatal("gRPC serve: " + err.Error())
		}
	}()

	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", metrics.Handler())
	metricsMux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	metricsSrv := &http.Server{
		Addr:              ":" + cfg.MetricsHTTPPort,
		Handler:           metricsMux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		appLogger.Info("Search metrics/health HTTP listening on :" + cfg.MetricsHTTPPort)
		if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			appLogger.Fatal("metrics HTTP: " + err.Error())
		}
	}()

	// Background work (index setup, then Kafka indexing) stops on shutdown.
	bgCtx, stopBackground := context.WithCancel(context.Background())
	defer stopBackground()

	var kafkaConsumer *kafka.Consumer
	if cfg.Kafka.Enabled && osClient != nil {
		kafkaConsumer = kafka.NewConsumer(
			cfg.Kafka.Brokers,
			cfg.Kafka.ConsumerGroup,
			cfg.Kafka.TopicUsers,
			cfg.Kafka.TopicPosts,
			cfg.Kafka.DLQTopic,
			cfg.UsersIndexName,
			cfg.PostsIndexName,
			cfg.Kafka.SASLUsername,
			cfg.Kafka.SASLPassword,
			time.Duration(cfg.Kafka.RetryBackoffMS)*time.Millisecond,
			osClient,
			appLogger,
		)
	} else {
		appLogger.Info("Kafka not configured; no async indexing")
	}

	// OpenSearch may come up after us: searches answer partial until then. The
	// consumer starts only once the indices exist, otherwise its first write
	// would auto-create an index with a dynamic mapping.
	if osClient != nil {
		go func() {
			if osClient.WaitForIndices(bgCtx, cfg.UsersIndexName, cfg.PostsIndexName, indicesRetryDelay) != nil {
				return // shutting down
			}
			if kafkaConsumer != nil {
				kafkaConsumer.Run(bgCtx)
			}
		}()
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	appLogger.Info("Shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
		appLogger.Warn("metrics HTTP shutdown: " + err.Error())
	}

	stopBackground()
	if kafkaConsumer != nil {
		if err := kafkaConsumer.Close(); err != nil {
			appLogger.Warn("Kafka consumer close: " + err.Error())
		}
	}

	grpcServer.GracefulStop()
	appLogger.Info("Done")
}

func unaryLoggingInterceptor(log *logger.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		log.Debug(info.FullMethod + " " + time.Since(start).String())
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

package main

import (
	"context"
	"errors"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	searchv1 "github.com/nikitashilov/microblog_grpc/proto/search/v1"
	"search-service/internal/application/services"
	"search-service/internal/config"
	"search-service/internal/infrastructure/kafka"
	"search-service/internal/infrastructure/opensearch"
	grpcinterface "search-service/internal/interfaces/grpc"

	"search-service/pkg/logger"

	"google.golang.org/grpc"
	grpc_reflection "google.golang.org/grpc/reflection"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	appLogger := logger.New(cfg.LogLevel)

	var osClient *opensearch.Client
	if cfg.OpenSearch.Enabled {
		osClient, err = opensearch.NewClient(cfg.OpenSearch.URL, appLogger)
		if err != nil {
			appLogger.Fatal("OpenSearch: " + err.Error())
		}
		if err := osClient.EnsureIndices(context.Background(), cfg.UsersIndexName, cfg.PostsIndexName); err != nil {
			appLogger.Warn("EnsureIndices: " + err.Error())
		}
	} else {
		appLogger.Info("OpenSearch not configured; search will return empty results")
	}

	searchSvc, err := services.NewSearchService(
		osClient,
		cfg.UsersIndexName,
		cfg.PostsIndexName,
		cfg.UserServiceGRPC,
		appLogger,
	)
	if err != nil {
		appLogger.Fatal("SearchService: " + err.Error())
	}
	defer searchSvc.Close()

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(unaryLoggingInterceptor(appLogger)),
	)
	searchv1.RegisterSearchServiceServer(grpcServer, grpcinterface.NewSearchServer(searchSvc, appLogger))
	grpc_reflection.Register(grpcServer)

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

	if cfg.Kafka.Enabled && osClient != nil {
		consumer := kafka.NewConsumer(
			cfg.Kafka.Brokers,
			cfg.Kafka.ConsumerGroup,
			cfg.Kafka.TopicUsers,
			cfg.Kafka.TopicPosts,
			cfg.UsersIndexName,
			cfg.PostsIndexName,
			osClient,
			appLogger,
		)
		defer consumer.Close()
		consumerCtx, stopConsumer := context.WithCancel(context.Background())
		go consumer.Run(consumerCtx)
		defer stopConsumer()
	} else {
		appLogger.Info("Kafka not configured; no async indexing")
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	appLogger.Info("Shutting down...")
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

package grpc

import (
	"context"

	searchv1 "github.com/nikitashilov/microblog_grpc/proto/search/v1"
	"search-service/internal/application/services"
	"search-service/pkg/logger"

	"google.golang.org/protobuf/types/known/emptypb"
)

type SearchServer struct {
	searchv1.UnimplementedSearchServiceServer
	svc  *services.SearchService
	log  *logger.Logger
}

func NewSearchServer(svc *services.SearchService, log *logger.Logger) *SearchServer {
	return &SearchServer{svc: svc, log: log}
}

func (s *SearchServer) Search(ctx context.Context, req *searchv1.SearchRequest) (*searchv1.SearchResponse, error) {
	return s.svc.Search(ctx, req)
}

func (s *SearchServer) HealthCheck(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

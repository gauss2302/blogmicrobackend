package clients

import (
	"context"
	"fmt"
	"time"

	"api-gateway/pkg/logger"

	searchv1 "github.com/nikitashilov/microblog_grpc/proto/search/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/protobuf/types/known/emptypb"
)

const defaultSearchTimeout = 15 * time.Second

type SearchClient struct {
	conn   *grpc.ClientConn
	client searchv1.SearchServiceClient
	logger *logger.Logger
}

func NewSearchClient(addr string, logger *logger.Logger) (*SearchClient, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                keepaliveTime,
			Timeout:             keepaliveTimeout,
			PermitWithoutStream: keepalivePermitWithoutStream,
		}),
		grpc.WithUnaryInterceptor(unaryClientLoggingInterceptor(logger)),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to search gRPC service: %w", err)
	}
	return &SearchClient{
		conn:   conn,
		client: searchv1.NewSearchServiceClient(conn),
		logger: logger,
	}, nil
}

func (c *SearchClient) Search(ctx context.Context, query, requestingUserID string, usersLimit, postsLimit int32, usersCursor, postsCursor string) (*searchv1.SearchResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultSearchTimeout)
	defer cancel()
	return c.client.Search(ctx, &searchv1.SearchRequest{
		Query:             query,
		RequestingUserId:  requestingUserID,
		UsersLimit:        usersLimit,
		PostsLimit:        postsLimit,
		UsersCursor:       usersCursor,
		PostsCursor:       postsCursor,
	})
}

func (c *SearchClient) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := c.client.HealthCheck(ctx, &emptypb.Empty{})
	return err
}

func (c *SearchClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

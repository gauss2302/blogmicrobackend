package clients

import (
	"context"
	"fmt"
	"time"

	"api-gateway/internal/models"
	"api-gateway/pkg/logger"

	postv1 "github.com/nikitashilov/microblog_grpc/proto/post/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const defaultPostTimeout = 10 * time.Second

// PostClient provides typed access to the post gRPC service.
type PostClient struct {
	conn   *grpc.ClientConn
	client postv1.PostServiceClient
	logger *logger.Logger
}

type CreatePostInput struct {
	UserID    string `json:"-"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Slug      string `json:"slug,omitempty"`
	Published bool   `json:"published,omitempty"`
}

type UpdatePostInput struct {
	ID        string  `json:"-"`
	UserID    string  `json:"-"`
	Title     *string `json:"title,omitempty"`
	Content   *string `json:"content,omitempty"`
	Slug      *string `json:"slug,omitempty"`
	Published *bool   `json:"published,omitempty"`
}

func NewPostClient(addr string, logger *logger.Logger) (*PostClient, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                keepaliveTime,
			Timeout:              keepaliveTimeout,
			PermitWithoutStream: keepalivePermitWithoutStream,
		}),
		grpc.WithUnaryInterceptor(unaryClientLoggingInterceptor(logger)),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to post gRPC service: %w", err)
	}

	return &PostClient{
		conn:   conn,
		client: postv1.NewPostServiceClient(conn),
		logger: logger,
	}, nil
}

func (c *PostClient) CreatePost(ctx context.Context, input *CreatePostInput) (*models.PostResponse, error) {
	if input == nil {
		return nil, fmt.Errorf("create post input is required")
	}

	ctx, cancel := context.WithTimeout(ctx, defaultPostTimeout)
	defer cancel()

	req := &postv1.CreatePostRequest{
		UserId:    input.UserID,
		Title:     input.Title,
		Content:   input.Content,
		Slug:      input.Slug,
		Published: input.Published,
	}

	resp, err := c.client.CreatePost(ctx, req)
	if err != nil {
		return nil, c.wrapError("create post", err)
	}

	return postFromProto(resp), nil
}

func (c *PostClient) GetPost(ctx context.Context, id, requestingUserID string) (*models.PostResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultPostTimeout)
	defer cancel()

	req := &postv1.GetPostRequest{Id: id, RequestingUserId: requestingUserID}
	resp, err := c.client.GetPost(ctx, req)
	if err != nil {
		return nil, c.wrapError("get post", err)
	}

	return postFromProto(resp), nil
}

func (c *PostClient) GetPostBySlug(ctx context.Context, slug string) (*models.PostResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultPostTimeout)
	defer cancel()

	resp, err := c.client.GetPostBySlug(ctx, &postv1.GetPostBySlugRequest{Slug: slug})
	if err != nil {
		return nil, c.wrapError("get post by slug", err)
	}

	return postFromProto(resp), nil
}

func (c *PostClient) UpdatePost(ctx context.Context, input *UpdatePostInput) (*models.PostResponse, error) {
	if input == nil {
		return nil, fmt.Errorf("update post input is required")
	}

	ctx, cancel := context.WithTimeout(ctx, defaultPostTimeout)
	defer cancel()

	req := &postv1.UpdatePostRequest{
		Id:     input.ID,
		UserId: input.UserID,
	}

	if input.Title != nil {
		req.Title = wrapperspb.String(*input.Title)
	}
	if input.Content != nil {
		req.Content = wrapperspb.String(*input.Content)
	}
	if input.Slug != nil {
		req.Slug = wrapperspb.String(*input.Slug)
	}
	if input.Published != nil {
		req.Published = wrapperspb.Bool(*input.Published)
	}

	resp, err := c.client.UpdatePost(ctx, req)
	if err != nil {
		return nil, c.wrapError("update post", err)
	}

	return postFromProto(resp), nil
}

func (c *PostClient) DeletePost(ctx context.Context, id, userID string) error {
	ctx, cancel := context.WithTimeout(ctx, defaultPostTimeout)
	defer cancel()

	req := &postv1.DeletePostRequest{Id: id, UserId: userID}
	if _, err := c.client.DeletePost(ctx, req); err != nil {
		return c.wrapError("delete post", err)
	}
	return nil
}

func (c *PostClient) ListPosts(ctx context.Context, limit, offset int, publishedOnly bool) (*models.ListPostsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultPostTimeout)
	defer cancel()

	req := &postv1.ListPostsRequest{Limit: int32(limit), Offset: int32(offset), PublishedOnly: publishedOnly}
	resp, err := c.client.ListPosts(ctx, req)
	if err != nil {
		return nil, c.wrapError("list posts", err)
	}

	return listPostsFromProto(resp), nil
}

func (c *PostClient) GetUserPosts(ctx context.Context, userID string, limit, offset int) (*models.ListPostsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultPostTimeout)
	defer cancel()

	req := &postv1.GetUserPostsRequest{UserId: userID, Limit: int32(limit), Offset: int32(offset)}
	resp, err := c.client.GetUserPosts(ctx, req)
	if err != nil {
		return nil, c.wrapError("get user posts", err)
	}

	return listPostsFromProto(resp), nil
}

func (c *PostClient) SearchPosts(ctx context.Context, query string, limit, offset int, publishedOnly bool) (*models.ListPostsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultPostTimeout)
	defer cancel()

	req := &postv1.SearchPostsRequest{Query: query, Limit: int32(limit), Offset: int32(offset), PublishedOnly: publishedOnly}
	resp, err := c.client.SearchPosts(ctx, req)
	if err != nil {
		return nil, c.wrapError("search posts", err)
	}

	return listPostsFromProto(resp), nil
}

func (c *PostClient) GetStats(ctx context.Context, userID string) (*models.PostStatsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultPostTimeout)
	defer cancel()

	resp, err := c.client.GetStats(ctx, &postv1.GetStatsRequest{UserId: userID})
	if err != nil {
		return nil, c.wrapError("get stats", err)
	}

	return &models.PostStatsResponse{
		TotalPublishedPosts: resp.GetTotalPublishedPosts(),
		UserPostsCount:      resp.GetUserPostsCount(),
	}, nil
}

func (c *PostClient) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if _, err := c.client.HealthCheck(ctx, &emptypb.Empty{}); err != nil {
		return c.wrapError("health check", err)
	}
	return nil
}

func (c *PostClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *PostClient) wrapError(action string, err error) error {
	if err == nil {
		return nil
	}

	if st, ok := status.FromError(err); ok {
		return status.Errorf(st.Code(), "%s: %s", action, st.Message())
	}

	return fmt.Errorf("%s: %w", action, err)
}

func postFromProto(p *postv1.Post) *models.PostResponse {
	if p == nil {
		return nil
	}

	return &models.PostResponse{
		ID:        p.GetId(),
		UserID:    p.GetUserId(),
		Title:     p.GetTitle(),
		Content:   p.GetContent(),
		Slug:      p.GetSlug(),
		Published: p.GetPublished(),
		CreatedAt: timestampToTime(p.GetCreatedAt()),
		UpdatedAt: timestampToTime(p.GetUpdatedAt()),
	}
}

func summaryFromProto(s *postv1.PostSummary) *models.PostSummaryResponse {
	if s == nil {
		return nil
	}

	return &models.PostSummaryResponse{
		ID:        s.GetId(),
		UserID:    s.GetUserId(),
		Title:     s.GetTitle(),
		Slug:      s.GetSlug(),
		Published: s.GetPublished(),
		CreatedAt: timestampToTime(s.GetCreatedAt()),
		UpdatedAt: timestampToTime(s.GetUpdatedAt()),
	}
}

func listPostsFromProto(resp *postv1.ListPostsResponse) *models.ListPostsResponse {
	if resp == nil {
		return nil
	}

	posts := make([]*models.PostSummaryResponse, 0, len(resp.GetPosts()))
	for _, summary := range resp.GetPosts() {
		posts = append(posts, summaryFromProto(summary))
	}

	return &models.ListPostsResponse{
		Posts:  posts,
		Limit:  int(resp.GetLimit()),
		Offset: int(resp.GetOffset()),
		Total:  int(resp.GetTotal()),
	}
}

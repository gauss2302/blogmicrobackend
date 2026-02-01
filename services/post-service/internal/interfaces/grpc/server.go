package grpc

import (
	"context"
	"net/http"
	"time"

	"post-service/internal/application/dto"
	appErrors "post-service/internal/application/errors"
	"post-service/internal/application/services"
	"post-service/pkg/logger"

	postv1 "github.com/nikitashilov/microblog_grpc/proto/post/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type PostServer struct {
	postv1.UnimplementedPostServiceServer
	service *services.PostService
	logger  *logger.Logger
}

func NewPostServer(service *services.PostService, logger *logger.Logger) *PostServer {
	return &PostServer{service: service, logger: logger}
}

func (s *PostServer) CreatePost(ctx context.Context, req *postv1.CreatePostRequest) (*postv1.Post, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.Unauthenticated, appErrors.ErrUnauthorizedAccess.Message)
	}

	dtoReq := &dto.CreatePostRequest{
		Title:     req.GetTitle(),
		Content:   req.GetContent(),
		Slug:      req.GetSlug(),
		Published: req.GetPublished(),
	}

	resp, err := s.service.CreatePost(ctx, dtoReq, req.GetUserId())
	if err != nil {
		return nil, s.toGRPCError(err)
	}

	return toProtoPost(resp), nil
}

func (s *PostServer) GetPost(ctx context.Context, req *postv1.GetPostRequest) (*postv1.Post, error) {
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, appErrors.ErrInvalidRequest.Message)
	}

	resp, err := s.service.GetPost(ctx, req.GetId(), req.GetRequestingUserId())
	if err != nil {
		return nil, s.toGRPCError(err)
	}

	return toProtoPost(resp), nil
}

func (s *PostServer) GetPostBySlug(ctx context.Context, req *postv1.GetPostBySlugRequest) (*postv1.Post, error) {
	if req.GetSlug() == "" {
		return nil, status.Error(codes.InvalidArgument, appErrors.ErrInvalidRequest.Message)
	}

	resp, err := s.service.GetPostBySlug(ctx, req.GetSlug())
	if err != nil {
		return nil, s.toGRPCError(err)
	}

	return toProtoPost(resp), nil
}

func (s *PostServer) UpdatePost(ctx context.Context, req *postv1.UpdatePostRequest) (*postv1.Post, error) {
	if req.GetId() == "" || req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, appErrors.ErrInvalidRequest.Message)
	}

	// Verify ownership - users can only update their own posts
	// The service layer will also check this, but we validate early for consistency with user-service pattern
	ownerID, err := s.service.GetPostOwner(ctx, req.GetId())
	if err != nil {
		return nil, s.toGRPCError(err)
	}

	if ownerID != req.GetUserId() {
		return nil, status.Error(codes.PermissionDenied, appErrors.ErrUnauthorizedAccess.Message)
	}

	dtoReq := &dto.UpdatePostRequest{}

	if req.GetTitle() != nil {
		value := req.GetTitle().GetValue()
		dtoReq.Title = &value
	}
	if req.GetContent() != nil {
		value := req.GetContent().GetValue()
		dtoReq.Content = &value
	}
	if req.GetSlug() != nil {
		value := req.GetSlug().GetValue()
		dtoReq.Slug = &value
	}
	if req.GetPublished() != nil {
		value := req.GetPublished().GetValue()
		dtoReq.Published = &value
	}

	resp, err := s.service.UpdatePost(ctx, req.GetId(), dtoReq, req.GetUserId())
	if err != nil {
		return nil, s.toGRPCError(err)
	}

	return toProtoPost(resp), nil
}

func (s *PostServer) DeletePost(ctx context.Context, req *postv1.DeletePostRequest) (*emptypb.Empty, error) {
	if req.GetId() == "" || req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, appErrors.ErrInvalidRequest.Message)
	}

	// Verify ownership - users can only delete their own posts
	// The service layer will also check this, but we validate early for consistency with user-service pattern
	ownerID, err := s.service.GetPostOwner(ctx, req.GetId())
	if err != nil {
		return nil, s.toGRPCError(err)
	}

	if ownerID != req.GetUserId() {
		return nil, status.Error(codes.PermissionDenied, appErrors.ErrUnauthorizedAccess.Message)
	}

	if err := s.service.DeletePost(ctx, req.GetId(), req.GetUserId()); err != nil {
		return nil, s.toGRPCError(err)
	}

	return &emptypb.Empty{}, nil
}

func (s *PostServer) ListPosts(ctx context.Context, req *postv1.ListPostsRequest) (*postv1.ListPostsResponse, error) {
	limit := normalizeLimit(int(req.GetLimit()))
	offset := normalizeOffset(int(req.GetOffset()))

	dtoReq := &dto.ListPostsRequest{
		Limit:         limit,
		Offset:        offset,
		PublishedOnly: req.GetPublishedOnly(),
	}

	resp, err := s.service.ListPosts(ctx, dtoReq)
	if err != nil {
		return nil, s.toGRPCError(err)
	}

	return toProtoListPosts(resp), nil
}

func (s *PostServer) GetUserPosts(ctx context.Context, req *postv1.GetUserPostsRequest) (*postv1.ListPostsResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, appErrors.ErrInvalidRequest.Message)
	}

	limit := normalizeLimit(int(req.GetLimit()))
	offset := normalizeOffset(int(req.GetOffset()))

	dtoReq := &dto.UserPostsRequest{
		Limit:  limit,
		Offset: offset,
	}

	resp, err := s.service.GetUserPosts(ctx, req.GetUserId(), dtoReq)
	if err != nil {
		return nil, s.toGRPCError(err)
	}

	return toProtoListPosts(resp), nil
}

func (s *PostServer) SearchPosts(ctx context.Context, req *postv1.SearchPostsRequest) (*postv1.ListPostsResponse, error) {
	if req.GetQuery() == "" {
		return nil, status.Error(codes.InvalidArgument, appErrors.ErrInvalidRequest.Message)
	}

	limit := normalizeLimit(int(req.GetLimit()))
	offset := normalizeOffset(int(req.GetOffset()))

	dtoReq := &dto.SearchPostsRequest{
		Query:         req.GetQuery(),
		Limit:         limit,
		Offset:        offset,
		PublishedOnly: req.GetPublishedOnly(),
	}

	resp, err := s.service.SearchPosts(ctx, dtoReq)
	if err != nil {
		return nil, s.toGRPCError(err)
	}

	return toProtoListPosts(resp), nil
}

func (s *PostServer) GetStats(ctx context.Context, req *postv1.GetStatsRequest) (*postv1.PostStatsResponse, error) {
	resp, err := s.service.GetStats(ctx, req.GetUserId())
	if err != nil {
		return nil, s.toGRPCError(err)
	}

	return &postv1.PostStatsResponse{
		TotalPublishedPosts: resp.TotalPublishedPosts,
		UserPostsCount:      resp.UserPostsCount,
	}, nil
}

func (s *PostServer) HealthCheck(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s *PostServer) toGRPCError(err error) error {
	if err == nil {
		return nil
	}

	if postErr, ok := err.(*appErrors.PostError); ok {
		switch postErr.StatusCode {
		case http.StatusBadRequest:
			return status.Error(codes.InvalidArgument, postErr.Message)
		case http.StatusUnauthorized:
			return status.Error(codes.Unauthenticated, postErr.Message)
		case http.StatusForbidden:
			return status.Error(codes.PermissionDenied, postErr.Message)
		case http.StatusNotFound:
			return status.Error(codes.NotFound, postErr.Message)
		case http.StatusConflict:
			return status.Error(codes.AlreadyExists, postErr.Message)
		case http.StatusTooManyRequests:
			return status.Error(codes.ResourceExhausted, postErr.Message)
		case http.StatusServiceUnavailable:
			return status.Error(codes.Unavailable, postErr.Message)
		default:
			return status.Error(codes.Internal, postErr.Message)
		}
	}

	s.logger.Error("unexpected error: " + err.Error())
	return status.Error(codes.Internal, "internal server error")
}

func toProtoPost(post *dto.PostResponse) *postv1.Post {
	if post == nil {
		return nil
	}

	return &postv1.Post{
		Id:        post.ID,
		UserId:    post.UserID,
		Title:     post.Title,
		Content:   post.Content,
		Slug:      post.Slug,
		Published: post.Published,
		CreatedAt: toTimestamp(post.CreatedAt),
		UpdatedAt: toTimestamp(post.UpdatedAt),
	}
}

func toProtoSummary(post *dto.PostSummaryResponse) *postv1.PostSummary {
	if post == nil {
		return nil
	}

	return &postv1.PostSummary{
		Id:        post.ID,
		UserId:    post.UserID,
		Title:     post.Title,
		Slug:      post.Slug,
		Published: post.Published,
		CreatedAt: toTimestamp(post.CreatedAt),
		UpdatedAt: toTimestamp(post.UpdatedAt),
	}
}

func toProtoListPosts(resp *dto.ListPostsResponse) *postv1.ListPostsResponse {
	if resp == nil {
		return nil
	}

	summaries := make([]*postv1.PostSummary, 0, len(resp.Posts))
	for _, summary := range resp.Posts {
		summaries = append(summaries, toProtoSummary(summary))
	}

	return &postv1.ListPostsResponse{
		Posts:  summaries,
		Limit:  int32(resp.Limit),
		Offset: int32(resp.Offset),
		Total:  int32(resp.Total),
	}
}

func toTimestamp(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

func normalizeLimit(limit int) int {
	if limit <= 0 || limit > 100 {
		return 20
	}
	return limit
}

func normalizeOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

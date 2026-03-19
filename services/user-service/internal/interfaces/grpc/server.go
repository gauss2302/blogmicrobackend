package grpc

import (
	"context"
	"net/http"
	"time"

	"user-service/internal/application/dto"
	appErrors "user-service/internal/application/errors"
	"user-service/internal/application/services"
	"user-service/pkg/logger"

	// userv1 "/microblog_grpc/proto/user/v1"
	userv1 "github.com/nikitashilov/microblog_grpc/proto/user/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// UserServer exposes user-domain functionality over gRPC.
type UserServer struct {
	userv1.UnimplementedUserServiceServer
	service *services.UserService
	logger  *logger.Logger
}

func NewUserServer(service *services.UserService, logger *logger.Logger) *UserServer {
	return &UserServer{service: service, logger: logger}
}

func (s *UserServer) CreateUser(ctx context.Context, req *userv1.CreateUserRequest) (*userv1.User, error) {
	dtoReq := &dto.CreateUserRequest{
		ID:       req.GetId(),
		Email:    req.GetEmail(),
		Name:     req.GetName(),
		Picture:  req.GetPicture(),
		Password: req.GetPassword(),
	}

	resp, err := s.service.CreateUser(ctx, dtoReq)
	if err != nil {
		return nil, s.toGRPCError(err)
	}

	return toProtoUser(resp), nil
}

func (s *UserServer) ValidateCredentials(ctx context.Context, req *userv1.ValidateCredentialsRequest) (*userv1.ValidateCredentialsResponse, error) {
	resp, err := s.service.ValidateCredentials(ctx, req.GetEmail(), req.GetPassword())
	if err != nil {
		return nil, s.toGRPCError(err)
	}

	return &userv1.ValidateCredentialsResponse{
		Id:      resp.ID,
		Email:   resp.Email,
		Name:    resp.Name,
		Picture: resp.Picture,
	}, nil
}

func (s *UserServer) GetUser(ctx context.Context, req *userv1.GetUserRequest) (*userv1.User, error) {
	resp, err := s.service.GetUser(ctx, req.GetId())
	if err != nil {
		return nil, s.toGRPCError(err)
	}

	return toProtoUser(resp), nil
}

func (s *UserServer) GetUserByEmail(ctx context.Context, req *userv1.GetUserByEmailRequest) (*userv1.User, error) {
	resp, err := s.service.GetUserByEmail(ctx, req.GetEmail())
	if err != nil {
		return nil, s.toGRPCError(err)
	}

	return toProtoUser(resp), nil
}

func (s *UserServer) GetUserProfile(ctx context.Context, req *userv1.GetUserProfileRequest) (*userv1.UserProfile, error) {
	resp, err := s.service.GetUserProfile(ctx, req.GetId())
	if err != nil {
		return nil, s.toGRPCError(err)
	}

	return toProtoUserProfile(resp), nil
}

func (s *UserServer) UpdateUser(ctx context.Context, req *userv1.UpdateUserRequest) (*userv1.User, error) {
	if req.GetActorId() == "" || req.GetActorId() != req.GetId() {
		return nil, status.Error(codes.PermissionDenied, appErrors.ErrUnauthorizedAccess.Message)
	}

	dtoReq := &dto.UpdateUserRequest{}

	if req.GetName() != nil {
		value := req.GetName().GetValue()
		dtoReq.Name = &value
	}
	if req.GetPicture() != nil {
		value := req.GetPicture().GetValue()
		dtoReq.Picture = &value
	}
	if req.GetBio() != nil {
		value := req.GetBio().GetValue()
		dtoReq.Bio = &value
	}
	if req.GetLocation() != nil {
		value := req.GetLocation().GetValue()
		dtoReq.Location = &value
	}
	if req.GetWebsite() != nil {
		value := req.GetWebsite().GetValue()
		dtoReq.Website = &value
	}

	resp, err := s.service.UpdateUser(ctx, req.GetId(), dtoReq)
	if err != nil {
		return nil, s.toGRPCError(err)
	}

	return toProtoUser(resp), nil
}

func (s *UserServer) DeleteUser(ctx context.Context, req *userv1.DeleteUserRequest) (*emptypb.Empty, error) {
	if req.GetActorId() == "" || req.GetActorId() != req.GetId() {
		return nil, status.Error(codes.PermissionDenied, appErrors.ErrUnauthorizedAccess.Message)
	}

	if err := s.service.DeleteUser(ctx, req.GetId()); err != nil {
		return nil, s.toGRPCError(err)
	}

	return &emptypb.Empty{}, nil
}

func (s *UserServer) ListUsers(ctx context.Context, req *userv1.ListUsersRequest) (*userv1.ListUsersResponse, error) {
	limit := int(req.GetLimit())
	offset := int(req.GetOffset())
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	dtoReq := &dto.ListUsersRequest{
		Limit:  limit,
		Offset: offset,
	}

	resp, err := s.service.ListUsers(ctx, dtoReq)
	if err != nil {
		return nil, s.toGRPCError(err)
	}

	return toProtoListUsers(resp), nil
}

func (s *UserServer) SearchUsers(ctx context.Context, req *userv1.SearchUsersRequest) (*userv1.ListUsersResponse, error) {
	if req.GetQuery() == "" {
		return nil, status.Error(codes.InvalidArgument, appErrors.ErrInvalidRequest.Message)
	}

	limit := int(req.GetLimit())
	offset := int(req.GetOffset())
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	dtoReq := &dto.SearchUsersRequest{
		Query:  req.GetQuery(),
		Limit:  limit,
		Offset: offset,
	}

	resp, err := s.service.SearchUsers(ctx, dtoReq)
	if err != nil {
		return nil, s.toGRPCError(err)
	}

	return toProtoListUsers(resp), nil
}

func (s *UserServer) GetStats(ctx context.Context, _ *emptypb.Empty) (*userv1.UserStatsResponse, error) {
	resp, err := s.service.GetStats(ctx)
	if err != nil {
		return nil, s.toGRPCError(err)
	}

	return &userv1.UserStatsResponse{TotalActiveUsers: resp.TotalActiveUsers}, nil
}

func (s *UserServer) Follow(ctx context.Context, req *userv1.FollowRequest) (*emptypb.Empty, error) {
	if req.GetFollowerId() == "" || req.GetFolloweeId() == "" {
		return nil, status.Error(codes.InvalidArgument, appErrors.ErrInvalidRequest.Message)
	}
	if err := s.service.Follow(ctx, req.GetFollowerId(), req.GetFolloweeId()); err != nil {
		return nil, s.toGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *UserServer) Unfollow(ctx context.Context, req *userv1.UnfollowRequest) (*emptypb.Empty, error) {
	if req.GetFollowerId() == "" || req.GetFolloweeId() == "" {
		return nil, status.Error(codes.InvalidArgument, appErrors.ErrInvalidRequest.Message)
	}
	if err := s.service.Unfollow(ctx, req.GetFollowerId(), req.GetFolloweeId()); err != nil {
		return nil, s.toGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *UserServer) GetFollowers(ctx context.Context, req *userv1.GetFollowersRequest) (*userv1.ListFollowResponse, error) {
	limit := int(req.GetLimit())
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	users, nextCursor, err := s.service.GetFollowers(ctx, req.GetUserId(), limit, req.GetCursor())
	if err != nil {
		return nil, s.toGRPCError(err)
	}
	profiles := make([]*userv1.UserProfile, 0, len(users))
	for _, u := range users {
		profiles = append(profiles, toProtoUserProfile(u))
	}
	return &userv1.ListFollowResponse{Users: profiles, NextCursor: nextCursor}, nil
}

func (s *UserServer) GetFollowing(ctx context.Context, req *userv1.GetFollowingRequest) (*userv1.ListFollowResponse, error) {
	limit := int(req.GetLimit())
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	users, nextCursor, err := s.service.GetFollowing(ctx, req.GetUserId(), limit, req.GetCursor())
	if err != nil {
		return nil, s.toGRPCError(err)
	}
	profiles := make([]*userv1.UserProfile, 0, len(users))
	for _, u := range users {
		profiles = append(profiles, toProtoUserProfile(u))
	}
	return &userv1.ListFollowResponse{Users: profiles, NextCursor: nextCursor}, nil
}

func (s *UserServer) AreFollowed(ctx context.Context, req *userv1.AreFollowedRequest) (*userv1.AreFollowedResponse, error) {
	ids, err := s.service.AreFollowed(ctx, req.GetFollowerId(), req.GetFolloweeIds())
	if err != nil {
		return nil, s.toGRPCError(err)
	}
	return &userv1.AreFollowedResponse{FollowedIds: ids}, nil
}

func (s *UserServer) HealthCheck(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s *UserServer) toGRPCError(err error) error {
	if err == nil {
		return nil
	}

	if userErr, ok := err.(*appErrors.UserError); ok {
		switch userErr.StatusCode {
		case http.StatusBadRequest:
			return status.Error(codes.InvalidArgument, userErr.Message)
		case http.StatusUnauthorized:
			return status.Error(codes.Unauthenticated, userErr.Message)
		case http.StatusForbidden:
			return status.Error(codes.PermissionDenied, userErr.Message)
		case http.StatusNotFound:
			return status.Error(codes.NotFound, userErr.Message)
		case http.StatusConflict:
			return status.Error(codes.AlreadyExists, userErr.Message)
		case http.StatusTooManyRequests:
			return status.Error(codes.ResourceExhausted, userErr.Message)
		case http.StatusServiceUnavailable:
			return status.Error(codes.Unavailable, userErr.Message)
		default:
			return status.Error(codes.Internal, userErr.Message)
		}
	}

	s.logger.Error("unexpected error: " + err.Error())
	return status.Error(codes.Internal, "internal server error")
}

func toProtoUser(user *dto.UserResponse) *userv1.User {
	if user == nil {
		return nil
	}

	return &userv1.User{
		Id:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		Picture:   user.Picture,
		Bio:       user.Bio,
		Location:  user.Location,
		Website:   user.Website,
		IsActive:  user.IsActive,
		CreatedAt: toTimestamp(user.CreatedAt),
		UpdatedAt: toTimestamp(user.UpdatedAt),
	}
}

func toProtoUserProfile(profile *dto.UserProfileResponse) *userv1.UserProfile {
	if profile == nil {
		return nil
	}

	return &userv1.UserProfile{
		Id:       profile.ID,
		Email:    profile.Email,
		Name:     profile.Name,
		Picture:  profile.Picture,
		Bio:      profile.Bio,
		Location: profile.Location,
		Website:  profile.Website,
	}
}

func toProtoListUsers(resp *dto.ListUsersResponse) *userv1.ListUsersResponse {
	if resp == nil {
		return nil
	}

	protoUsers := make([]*userv1.User, 0, len(resp.Users))
	for _, user := range resp.Users {
		protoUsers = append(protoUsers, &userv1.User{
			Id:        user.ID,
			Email:     user.Email,
			Name:      user.Name,
			Picture:   user.Picture,
			Bio:       user.Bio,
			Location:  user.Location,
			Website:   user.Website,
			IsActive:  user.IsActive,
			CreatedAt: toTimestamp(user.CreatedAt),
			UpdatedAt: toTimestamp(user.UpdatedAt),
		})
	}

	return &userv1.ListUsersResponse{
		Users:  protoUsers,
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

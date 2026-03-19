package clients

import (
	"context"
	"fmt"
	"time"

	"api-gateway/internal/models"
	"api-gateway/pkg/logger"

	userv1 "github.com/nikitashilov/microblog_grpc/proto/user/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const defaultUserTimeout = 10 * time.Second

// UserClient wraps gRPC communication with the user service.
type UserClient struct {
	conn   *grpc.ClientConn
	client userv1.UserServiceClient
	logger *logger.Logger
}

type CreateUserInput struct {
	ID      string `json:"id,omitempty"`
	Email   string `json:"email" binding:"required"`
	Name    string `json:"name" binding:"required"`
	Picture string `json:"picture,omitempty"`
}

type UpdateUserInput struct {
	ID       string  `json:"-"`
	ActorID  string  `json:"-"`
	Name     *string `json:"name,omitempty"`
	Picture  *string `json:"picture,omitempty"`
	Bio      *string `json:"bio,omitempty"`
	Location *string `json:"location,omitempty"`
	Website  *string `json:"website,omitempty"`
}

func NewUserClient(addr string, logger *logger.Logger) (*UserClient, error) {
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
		return nil, fmt.Errorf("connect to user gRPC service: %w", err)
	}

	return &UserClient{
		conn:   conn,
		client: userv1.NewUserServiceClient(conn),
		logger: logger,
	}, nil
}

func (c *UserClient) CreateUser(ctx context.Context, input *CreateUserInput) (*models.UserResponse, error) {
	if input == nil {
		return nil, fmt.Errorf("create user input is required")
	}

	ctx, cancel := context.WithTimeout(ctx, defaultUserTimeout)
	defer cancel()

	req := &userv1.CreateUserRequest{
		Id:      input.ID,
		Email:   input.Email,
		Name:    input.Name,
		Picture: input.Picture,
	}

	resp, err := c.client.CreateUser(ctx, req)
	if err != nil {
		return nil, c.wrapError("create user", err)
	}

	return userFromProto(resp), nil
}

func (c *UserClient) GetUser(ctx context.Context, id string) (*models.UserResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultUserTimeout)
	defer cancel()

	resp, err := c.client.GetUser(ctx, &userv1.GetUserRequest{Id: id})
	if err != nil {
		return nil, c.wrapError("get user", err)
	}

	return userFromProto(resp), nil
}

func (c *UserClient) GetUserProfile(ctx context.Context, id string) (*models.UserProfileResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultUserTimeout)
	defer cancel()

	resp, err := c.client.GetUserProfile(ctx, &userv1.GetUserProfileRequest{Id: id})
	if err != nil {
		return nil, c.wrapError("get user profile", err)
	}

	return userProfileFromProto(resp), nil
}

func (c *UserClient) UpdateUser(ctx context.Context, input *UpdateUserInput) (*models.UserResponse, error) {
	if input == nil {
		return nil, fmt.Errorf("update user input is required")
	}

	ctx, cancel := context.WithTimeout(ctx, defaultUserTimeout)
	defer cancel()

	req := &userv1.UpdateUserRequest{
		Id:      input.ID,
		ActorId: input.ActorID,
	}

	if input.Name != nil {
		req.Name = wrapperspb.String(*input.Name)
	}
	if input.Picture != nil {
		req.Picture = wrapperspb.String(*input.Picture)
	}
	if input.Bio != nil {
		req.Bio = wrapperspb.String(*input.Bio)
	}
	if input.Location != nil {
		req.Location = wrapperspb.String(*input.Location)
	}
	if input.Website != nil {
		req.Website = wrapperspb.String(*input.Website)
	}

	resp, err := c.client.UpdateUser(ctx, req)
	if err != nil {
		return nil, c.wrapError("update user", err)
	}

	return userFromProto(resp), nil
}

func (c *UserClient) DeleteUser(ctx context.Context, id, actorID string) error {
	ctx, cancel := context.WithTimeout(ctx, defaultUserTimeout)
	defer cancel()

	req := &userv1.DeleteUserRequest{Id: id, ActorId: actorID}
	if _, err := c.client.DeleteUser(ctx, req); err != nil {
		return c.wrapError("delete user", err)
	}

	return nil
}

func (c *UserClient) ListUsers(ctx context.Context, limit, offset int) (*models.ListUsersResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultUserTimeout)
	defer cancel()

	req := &userv1.ListUsersRequest{Limit: int32(limit), Offset: int32(offset)}
	resp, err := c.client.ListUsers(ctx, req)
	if err != nil {
		return nil, c.wrapError("list users", err)
	}

	return listUsersFromProto(resp), nil
}

func (c *UserClient) SearchUsers(ctx context.Context, query string, limit, offset int) (*models.ListUsersResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultUserTimeout)
	defer cancel()

	req := &userv1.SearchUsersRequest{Query: query, Limit: int32(limit), Offset: int32(offset)}
	resp, err := c.client.SearchUsers(ctx, req)
	if err != nil {
		return nil, c.wrapError("search users", err)
	}

	return listUsersFromProto(resp), nil
}

func (c *UserClient) GetStats(ctx context.Context) (*models.UserStatsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultUserTimeout)
	defer cancel()

	resp, err := c.client.GetStats(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, c.wrapError("get stats", err)
	}

	return &models.UserStatsResponse{TotalActiveUsers: resp.GetTotalActiveUsers()}, nil
}

func (c *UserClient) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if _, err := c.client.HealthCheck(ctx, &emptypb.Empty{}); err != nil {
		return c.wrapError("health check", err)
	}

	return nil
}

func (c *UserClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *UserClient) wrapError(action string, err error) error {
	if err == nil {
		return nil
	}

	if st, ok := status.FromError(err); ok {
		return status.Errorf(st.Code(), "%s: %s", action, st.Message())
	}

	return fmt.Errorf("%s: %w", action, err)
}

func userFromProto(u *userv1.User) *models.UserResponse {
	if u == nil {
		return nil
	}

	return &models.UserResponse{
		ID:        u.GetId(),
		Email:     u.GetEmail(),
		Name:      u.GetName(),
		Picture:   u.GetPicture(),
		Bio:       u.GetBio(),
		Location:  u.GetLocation(),
		Website:   u.GetWebsite(),
		IsActive:  u.GetIsActive(),
		CreatedAt: timestampToTime(u.GetCreatedAt()),
		UpdatedAt: timestampToTime(u.GetUpdatedAt()),
	}
}

func userProfileFromProto(p *userv1.UserProfile) *models.UserProfileResponse {
	if p == nil {
		return nil
	}

	return &models.UserProfileResponse{
		ID:       p.GetId(),
		Email:    p.GetEmail(),
		Name:     p.GetName(),
		Picture:  p.GetPicture(),
		Bio:      p.GetBio(),
		Location: p.GetLocation(),
		Website:  p.GetWebsite(),
	}
}

func listUsersFromProto(resp *userv1.ListUsersResponse) *models.ListUsersResponse {
	if resp == nil {
		return nil
	}

	users := make([]*models.UserResponse, 0, len(resp.GetUsers()))
	for _, u := range resp.GetUsers() {
		users = append(users, userFromProto(u))
	}

	return &models.ListUsersResponse{
		Users:  users,
		Limit:  int(resp.GetLimit()),
		Offset: int(resp.GetOffset()),
		Total:  int(resp.GetTotal()),
	}
}

func timestampToTime(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.AsTime()
}

func (c *UserClient) Follow(ctx context.Context, followerID, followeeID string) error {
	ctx, cancel := context.WithTimeout(ctx, defaultUserTimeout)
	defer cancel()
	_, err := c.client.Follow(ctx, &userv1.FollowRequest{FollowerId: followerID, FolloweeId: followeeID})
	return err
}

func (c *UserClient) Unfollow(ctx context.Context, followerID, followeeID string) error {
	ctx, cancel := context.WithTimeout(ctx, defaultUserTimeout)
	defer cancel()
	_, err := c.client.Unfollow(ctx, &userv1.UnfollowRequest{FollowerId: followerID, FolloweeId: followeeID})
	return err
}

func (c *UserClient) GetFollowers(ctx context.Context, userID string, limit int, cursor string) (*models.ListFollowResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultUserTimeout)
	defer cancel()
	resp, err := c.client.GetFollowers(ctx, &userv1.GetFollowersRequest{UserId: userID, Limit: int32(limit), Cursor: cursor})
	if err != nil {
		return nil, c.wrapError("get followers", err)
	}
	return listFollowFromProto(resp), nil
}

func (c *UserClient) GetFollowing(ctx context.Context, userID string, limit int, cursor string) (*models.ListFollowResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultUserTimeout)
	defer cancel()
	resp, err := c.client.GetFollowing(ctx, &userv1.GetFollowingRequest{UserId: userID, Limit: int32(limit), Cursor: cursor})
	if err != nil {
		return nil, c.wrapError("get following", err)
	}
	return listFollowFromProto(resp), nil
}

func listFollowFromProto(resp *userv1.ListFollowResponse) *models.ListFollowResponse {
	if resp == nil {
		return nil
	}
	users := make([]*models.UserProfileResponse, 0, len(resp.GetUsers()))
	for _, u := range resp.GetUsers() {
		users = append(users, userProfileFromProto(u))
	}
	return &models.ListFollowResponse{Users: users, NextCursor: resp.GetNextCursor()}
}

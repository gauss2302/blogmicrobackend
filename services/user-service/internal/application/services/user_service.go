package services

import (
	"context"
	"crypto/rand"
	stderrors "errors"
	"fmt"
	"strings"
	"sync"

	"user-service/internal/application/dto"
	"user-service/internal/application/errors"
	"user-service/internal/domain/entities"
	"user-service/internal/domain/repositories"
	"user-service/pkg/logger"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// UserService has no search indexer: the users table's triggers queue every
// change for search (search.Relay sends it), so writes never wait for Kafka.
type UserService struct {
	userRepo   repositories.UserRepository
	followRepo repositories.FollowRepository
	logger     *logger.Logger
}

func NewUserService(userRepo repositories.UserRepository, followRepo repositories.FollowRepository, logger *logger.Logger) *UserService {
	return &UserService{
		userRepo:   userRepo,
		followRepo: followRepo,
		logger:     logger,
	}
}

// normalizeEmail matches entities.User.Sanitize, which lowercases email on
// write. Lookups must use the same form, or mixed-case input never matches.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (s *UserService) CreateUser(ctx context.Context, req *dto.CreateUserRequest) (*dto.UserResponse, error) {
	email := normalizeEmail(req.Email)
	s.logger.Info("Creating user")

	id := strings.TrimSpace(req.ID)
	if id == "" {
		id = uuid.New().String()
	}

	user := &entities.User{
		ID:       id,
		Email:    email,
		Name:     req.Name,
		Picture:  req.Picture,
		IsActive: true,
	}

	// Validate before the email lookup: input Postgres rejects (NUL) must be
	// a 400, not a failed SELECT read as the database being down (A-80).
	user.Sanitize()
	if err := user.IsValid(); err != nil {
		s.logger.Warn(fmt.Sprintf("User validation failed: %v", err))
		return nil, errors.ErrInvalidUserData
	}

	// Enforce password policy on the receiving service: the gRPC CreateUser
	// path used by real signups does not run the HTTP-layer validator, and
	// bcrypt silently truncates input past 72 bytes — reject rather than hash
	// only a prefix.
	if req.Password != "" && (len(req.Password) < 8 || len(req.Password) > 72) {
		s.logger.Warn("Password rejected: must be between 8 and 72 bytes")
		return nil, errors.ErrInvalidPassword
	}

	// Check if user already exists
	existingUser, err := s.userRepo.GetByEmail(ctx, email)
	if err == nil && existingUser != nil {
		return nil, errors.ErrUserAlreadyExists
	}
	if err != nil && !stderrors.Is(err, repositories.ErrUserNotFound) {
		return nil, s.lookupError(err, "check email before create")
	}

	if req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			s.logger.Error(fmt.Sprintf("Failed to hash password: %v", err))
			return nil, errors.ErrUserCreationFailed
		}
		user.PasswordHash = string(hash)
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		// Lost a signup race, or the email belongs to a soft-deleted account.
		if stderrors.Is(err, repositories.ErrUserAlreadyExists) {
			return nil, errors.ErrUserAlreadyExists
		}
		s.logger.Error(fmt.Sprintf("Failed to create user: %v", err))
		return nil, errors.ErrUserCreationFailed
	}

	s.logger.Info(fmt.Sprintf("User created successfully: %s", user.ID))

	return &dto.UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		Picture:   user.Picture,
		IsActive:  user.IsActive,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}, nil
}

func (s *UserService) GetUser(ctx context.Context, id string) (*dto.UserResponse, error) {
	s.logger.Info(fmt.Sprintf("Getting user: %s", id))

	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, s.lookupError(err, "get user "+id)
	}

	return &dto.UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		Picture:   user.Picture,
		Bio:       user.Bio,
		Location:  user.Location,
		Website:   user.Website,
		IsActive:  user.IsActive,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}, nil
}

func (s *UserService) GetUserByEmail(ctx context.Context, email string) (*dto.UserResponse, error) {
	email = normalizeEmail(email)
	s.logger.Debug("Getting user by email")

	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, s.lookupError(err, "get user by email")
	}

	return &dto.UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		Picture:   user.Picture,
		Bio:       user.Bio,
		Location:  user.Location,
		Website:   user.Website,
		IsActive:  user.IsActive,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}, nil
}

func (s *UserService) GetUserProfile(ctx context.Context, id string) (*dto.UserProfileResponse, error) {
	s.logger.Info(fmt.Sprintf("Getting user profile: %s", id))

	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, s.lookupError(err, "get user "+id)
	}

	profile := user.ToProfile()
	return &dto.UserProfileResponse{
		ID:       profile.ID,
		Email:    profile.Email,
		Name:     profile.Name,
		Picture:  profile.Picture,
		Bio:      profile.Bio,
		Location: profile.Location,
		Website:  profile.Website,
	}, nil
}

func (s *UserService) UpdateUser(ctx context.Context, id string, req *dto.UpdateUserRequest) (*dto.UserResponse, error) {
	s.logger.Info(fmt.Sprintf("Updating user: %s", id))

	// Get existing user
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, s.lookupError(err, "get user for update "+id)
	}

	// Update fields
	if req.Name != nil {
		user.Name = *req.Name
	}
	if req.Picture != nil {
		user.Picture = *req.Picture
	}
	if req.Bio != nil {
		user.Bio = *req.Bio
	}
	if req.Location != nil {
		user.Location = *req.Location
	}
	if req.Website != nil {
		user.Website = *req.Website
	}

	// Validate and sanitize
	user.Sanitize()
	if err := user.IsValid(); err != nil {
		s.logger.Warn(fmt.Sprintf("User validation failed on update: %v", err))
		return nil, errors.ErrInvalidUserData
	}

	// Update in database
	if err := s.userRepo.Update(ctx, user); err != nil {
		s.logger.Error(fmt.Sprintf("Failed to update user: %v", err))
		return nil, errors.ErrUserUpdateFailed
	}

	s.logger.Info(fmt.Sprintf("User updated successfully: %s", user.ID))

	return &dto.UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		Picture:   user.Picture,
		Bio:       user.Bio,
		Location:  user.Location,
		Website:   user.Website,
		IsActive:  user.IsActive,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}, nil
}

func (s *UserService) DeleteUser(ctx context.Context, id string) error {
	s.logger.Info(fmt.Sprintf("Deleting user: %s", id))

	if err := s.userRepo.Delete(ctx, id); err != nil {
		s.logger.Error(fmt.Sprintf("Failed to delete user: %v", err))
		return errors.ErrUserDeletionFailed
	}

	s.logger.Info(fmt.Sprintf("User deleted successfully: %s", id))
	return nil
}

func (s *UserService) ListUsers(ctx context.Context, req *dto.ListUsersRequest) (*dto.ListUsersResponse, error) {
	s.logger.Info(fmt.Sprintf("Listing users: limit=%d, offset=%d", req.Limit, req.Offset))

	users, err := s.userRepo.List(ctx, req.Limit, req.Offset)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Failed to list users: %v", err))
		return nil, errors.ErrUserListFailed
	}

	var userResponses []*dto.UserResponse
	for _, user := range users {
		userResponses = append(userResponses, &dto.UserResponse{
			ID:        user.ID,
			Email:     user.Email,
			Name:      user.Name,
			Picture:   user.Picture,
			Bio:       user.Bio,
			Location:  user.Location,
			Website:   user.Website,
			IsActive:  user.IsActive,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
		})
	}

	return &dto.ListUsersResponse{
		Users:  userResponses,
		Limit:  req.Limit,
		Offset: req.Offset,
		Total:  len(userResponses),
	}, nil
}

func (s *UserService) SearchUsers(ctx context.Context, req *dto.SearchUsersRequest) (*dto.ListUsersResponse, error) {
	s.logger.Info(fmt.Sprintf("Searching users: query length=%d, limit=%d, offset=%d", len(req.Query), req.Limit, req.Offset))

	users, err := s.userRepo.Search(ctx, req.Query, req.Limit, req.Offset)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Failed to search users: %v", err))
		return nil, errors.ErrUserSearchFailed
	}

	var userResponses []*dto.UserResponse
	for _, user := range users {
		userResponses = append(userResponses, &dto.UserResponse{
			ID:        user.ID,
			Email:     user.Email,
			Name:      user.Name,
			Picture:   user.Picture,
			Bio:       user.Bio,
			Location:  user.Location,
			Website:   user.Website,
			IsActive:  user.IsActive,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
		})
	}

	return &dto.ListUsersResponse{
		Users:  userResponses,
		Limit:  req.Limit,
		Offset: req.Offset,
		Total:  len(userResponses),
	}, nil
}

func (s *UserService) GetStats(ctx context.Context) (*dto.UserStatsResponse, error) {
	s.logger.Info("Getting user statistics")

	count, err := s.userRepo.GetActiveUsersCount(ctx)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Failed to get user stats: %v", err))
		return nil, errors.ErrUserStatsFailed
	}

	return &dto.UserStatsResponse{
		TotalActiveUsers: count,
	}, nil
}

// lookupError maps a failed GetByID/GetByEmail: a missing user is not found,
// anything else is the database failing and must not read as "not found".
func (s *UserService) lookupError(err error, what string) error {
	if stderrors.Is(err, repositories.ErrUserNotFound) {
		return errors.ErrUserNotFound
	}
	s.logger.Error(fmt.Sprintf("%s: %v", what, err))
	return errors.ErrServiceUnavailable
}

// dummyPasswordHash is a hash of a random password at the cost real ones use,
// compared against when there is no real hash to check.
var dummyPasswordHash = sync.OnceValue(func() []byte {
	h, err := bcrypt.GenerateFromPassword([]byte(rand.Text()), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return h
})

func (s *UserService) ValidateCredentials(ctx context.Context, email, password string) (*dto.ValidateCredentialsResponse, error) {
	email = normalizeEmail(email)
	s.logger.Debug("Validating credentials")

	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil && !stderrors.Is(err, repositories.ErrUserNotFound) {
		// The database failing is not a wrong password: 503, not 401.
		return nil, s.lookupError(err, "validate credentials")
	}
	if err != nil || user == nil || user.PasswordHash == "" {
		// Unknown email or an OAuth-only account: still pay for one bcrypt
		// comparison, so the response time does not tell which emails exist.
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash(), []byte(password))
		return nil, errors.ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		s.logger.Warn("Invalid password")
		return nil, errors.ErrInvalidCredentials
	}

	return &dto.ValidateCredentialsResponse{
		ID:      user.ID,
		Email:   user.Email,
		Name:    user.Name,
		Picture: user.Picture,
	}, nil
}

func (s *UserService) Follow(ctx context.Context, followerID, followeeID string) error {
	if followerID == followeeID {
		return errors.ErrCannotFollowSelf
	}
	if _, err := s.userRepo.GetByID(ctx, followeeID); err != nil {
		return s.lookupError(err, "get followee "+followeeID)
	}
	if err := s.followRepo.Create(ctx, followerID, followeeID); err != nil {
		s.logger.Error(fmt.Sprintf("Follow create: %v", err))
		return errors.ErrUserUpdateFailed
	}
	return nil
}

func (s *UserService) Unfollow(ctx context.Context, followerID, followeeID string) error {
	if err := s.followRepo.Delete(ctx, followerID, followeeID); err != nil {
		s.logger.Error(fmt.Sprintf("Unfollow: %v", err))
		return errors.ErrUserUpdateFailed
	}
	return nil
}

func (s *UserService) GetFollowers(ctx context.Context, userID string, limit int, cursor string) ([]*dto.UserProfileResponse, string, error) {
	users, nextCursor, err := s.followRepo.GetFollowers(ctx, userID, limit, cursor)
	if err != nil {
		s.logger.Error(fmt.Sprintf("GetFollowers: %v", err))
		return nil, "", errors.ErrUserListFailed
	}
	out := make([]*dto.UserProfileResponse, 0, len(users))
	for _, u := range users {
		out = append(out, &dto.UserProfileResponse{
			ID:       u.ID,
			Email:    u.Email,
			Name:     u.Name,
			Picture:  u.Picture,
			Bio:      u.Bio,
			Location: u.Location,
			Website:  u.Website,
		})
	}
	return out, nextCursor, nil
}

func (s *UserService) GetFollowing(ctx context.Context, userID string, limit int, cursor string) ([]*dto.UserProfileResponse, string, error) {
	users, nextCursor, err := s.followRepo.GetFollowing(ctx, userID, limit, cursor)
	if err != nil {
		s.logger.Error(fmt.Sprintf("GetFollowing: %v", err))
		return nil, "", errors.ErrUserListFailed
	}
	out := make([]*dto.UserProfileResponse, 0, len(users))
	for _, u := range users {
		out = append(out, &dto.UserProfileResponse{
			ID:       u.ID,
			Email:    u.Email,
			Name:     u.Name,
			Picture:  u.Picture,
			Bio:      u.Bio,
			Location: u.Location,
			Website:  u.Website,
		})
	}
	return out, nextCursor, nil
}

func (s *UserService) AreFollowed(ctx context.Context, followerID string, followeeIDs []string) ([]string, error) {
	if len(followeeIDs) == 0 {
		return nil, nil
	}
	return s.followRepo.AreFollowed(ctx, followerID, followeeIDs)
}

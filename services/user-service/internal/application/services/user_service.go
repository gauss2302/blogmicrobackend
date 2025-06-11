package services

import (
	"context"
	"fmt"

	"user-service/internal/application/dto"
	"user-service/internal/application/errors"
	"user-service/internal/domain/entities"
	"user-service/internal/domain/repositories"
	"user-service/pkg/logger"
)

type UserService struct {
	userRepo repositories.UserRepository
	logger   *logger.Logger
}

func NewUserService(userRepo repositories.UserRepository, logger *logger.Logger) *UserService {
	return &UserService{
		userRepo: userRepo,
		logger:   logger,
	}
}

func (s *UserService) CreateUser(ctx context.Context, req *dto.CreateUserRequest) (*dto.UserResponse, error) {
	s.logger.Info(fmt.Sprintf("Creating user with email: %s", req.Email))

	// Check if user already exists
	existingUser, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err == nil && existingUser != nil {
		return nil, errors.ErrUserAlreadyExists
	}

	// Create user entity
	user := &entities.User{
		ID:       req.ID,
		Email:    req.Email,
		Name:     req.Name,
		Picture:  req.Picture,
		IsActive: true,
	}

	// Validate and sanitize
	user.Sanitize()
	if err := user.IsValid(); err != nil {
		s.logger.Warn(fmt.Sprintf("User validation failed: %v", err))
		return nil, errors.ErrInvalidUserData
	}

	// Save to database
	if err := s.userRepo.Create(ctx, user); err != nil {
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
		s.logger.Warn(fmt.Sprintf("User not found: %s", id))
		return nil, errors.ErrUserNotFound
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
		s.logger.Warn(fmt.Sprintf("User not found: %s", id))
		return nil, errors.ErrUserNotFound
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
		s.logger.Warn(fmt.Sprintf("User not found for update: %s", id))
		return nil, errors.ErrUserNotFound
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
	s.logger.Info(fmt.Sprintf("Searching users: query=%s, limit=%d, offset=%d", req.Query, req.Limit, req.Offset))

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
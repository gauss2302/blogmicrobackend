package dto

import (
	"time"
)

type CreateUserRequest struct {
	ID      string `json:"id" binding:"required"`
	Email   string `json:"email" binding:"required,email"`
	Name    string `json:"name" binding:"required,min=1,max=100"`
	Picture string `json:"picture,omitempty"`
}

type UpdateUserRequest struct {
	Name     *string `json:"name,omitempty" binding:"omitempty,min=1,max=100"`
	Picture  *string `json:"picture,omitempty"`
	Bio      *string `json:"bio,omitempty" binding:"omitempty,max=500"`
	Location *string `json:"location,omitempty" binding:"omitempty,max=100"`
	Website  *string `json:"website,omitempty" binding:"omitempty,url"`
}

type UserResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Picture   string    `json:"picture,omitempty"`
	Bio       string    `json:"bio,omitempty"`
	Location  string    `json:"location,omitempty"`
	Website   string    `json:"website,omitempty"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UserProfileResponse struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Picture  string `json:"picture,omitempty"`
	Bio      string `json:"bio,omitempty"`
	Location string `json:"location,omitempty"`
	Website  string `json:"website,omitempty"`
}

type ListUsersRequest struct {
	Limit  int `form:"limit,default=20" binding:"omitempty,min=1,max=100"`
	Offset int `form:"offset,default=0" binding:"omitempty,min=0"`
}

type SearchUsersRequest struct {
	Query  string `form:"q" binding:"required,min=1"`
	Limit  int    `form:"limit,default=20" binding:"omitempty,min=1,max=100"`
	Offset int    `form:"offset,default=0" binding:"omitempty,min=0"`
}

type ListUsersResponse struct {
	Users  []*UserResponse `json:"users"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
	Total  int             `json:"total"`
}

type UserStatsResponse struct {
	TotalActiveUsers int64 `json:"total_active_users"`
}
package models

import "time"

// User models
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

// UserProfileResponse is the public/discovery view of a user. It deliberately
// omits email and other PII so it can be served on unauthenticated endpoints
// (public profile, search, follower/following lists).
type UserProfileResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Picture  string `json:"picture,omitempty"`
	Bio      string `json:"bio,omitempty"`
	Location string `json:"location,omitempty"`
	Website  string `json:"website,omitempty"`
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

type ListFollowResponse struct {
	Users      []*UserProfileResponse `json:"users"`
	NextCursor string                 `json:"next_cursor,omitempty"`
}

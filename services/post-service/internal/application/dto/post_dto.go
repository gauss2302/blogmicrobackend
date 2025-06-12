package dto

import (
	"time"
)

type CreatePostRequest struct {
	Title     string `json:"title" binding:"required,min=1,max=200"`
	Content   string `json:"content" binding:"required,min=1,max=50000"`
	Slug      string `json:"slug,omitempty" binding:"omitempty,min=3,max=100"`
	Published bool   `json:"published,omitempty"`
}

type UpdatePostRequest struct {
	Title     *string `json:"title,omitempty" binding:"omitempty,min=1,max=200"`
	Content   *string `json:"content,omitempty" binding:"omitempty,min=1,max=50000"`
	Slug      *string `json:"slug,omitempty" binding:"omitempty,min=3,max=100"`
	Published *bool   `json:"published,omitempty"`
}

type PostResponse struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Slug      string    `json:"slug"`
	Published bool      `json:"published"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PostSummaryResponse struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Title     string    `json:"title"`
	Slug      string    `json:"slug"`
	Published bool      `json:"published"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ListPostsRequest struct {
	Limit         int  `form:"limit,default=20" binding:"omitempty,min=1,max=100"`
	Offset        int  `form:"offset,default=0" binding:"omitempty,min=0"`
	PublishedOnly bool `form:"published_only,default=false"`
}

type SearchPostsRequest struct {
	Query         string `form:"q" binding:"required,min=1"`
	Limit         int    `form:"limit,default=20" binding:"omitempty,min=1,max=100"`
	Offset        int    `form:"offset,default=0" binding:"omitempty,min=0"`
	PublishedOnly bool   `form:"published_only,default=true"`
}

type UserPostsRequest struct {
	Limit  int `form:"limit,default=20" binding:"omitempty,min=1,max=100"`
	Offset int `form:"offset,default=0" binding:"omitempty,min=0"`
}

type ListPostsResponse struct {
	Posts  []*PostSummaryResponse `json:"posts"`
	Limit  int                    `json:"limit"`
	Offset int                    `json:"offset"`
	Total  int                    `json:"total"`
}

type PostStatsResponse struct {
	TotalPublishedPosts int64 `json:"total_published_posts"`
	UserPostsCount      int64 `json:"user_posts_count,omitempty"`
}
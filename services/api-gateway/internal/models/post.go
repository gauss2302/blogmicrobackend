package models

import "time"

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
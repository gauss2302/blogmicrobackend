package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"post-service/internal/domain/entities"
	"strings"
	"time"
)

type PostRepository struct {
	db *sql.DB
}

func NewPostRepository(db *sql.DB) *PostRepository {
	return &PostRepository{db: db}
}

func (r *PostRepository) Create(ctx context.Context, post *entities.Post) error {
	query := `
		INSERT INTO posts (id, user_id, title, content, slug, published, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	now := time.Now()
	_, err := r.db.ExecContext(ctx, query, post.ID, post.UserID, post.Title, post.Content, post.Slug, post.Published, now, now)

	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			if strings.Contains(err.Error(), "slug") {
				return fmt.Errorf("post with slug %s already exists", post.Slug)
			}
			return fmt.Errorf("post already exists")
		}
		return fmt.Errorf("failed to create post: %w", err)
	}

	post.CreatedAt = now
	post.UpdatedAt = now

	return nil
}

func (r *PostRepository) GetByID(ctx context.Context, id string) (*entities.Post, error) {
	query := `
		SELECT id, user_id, title, content, slug, published, created_at, updated_at
		FROM posts 
		WHERE id = $1
	`

	post := &entities.Post{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&post.ID, &post.UserID, &post.Title, &post.Content, &post.Slug,
		&post.Published, &post.CreatedAt, &post.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("post not found")
		}
		return nil, fmt.Errorf("failed to get post: %w", err)
	}

	return post, nil
}

func (r *PostRepository) GetBySlug(ctx context.Context, slug string) (*entities.Post, error) {
	query := `
		SELECT id, user_id, title, content, slug, published, created_at, updated_at
		FROM posts 
		WHERE slug = $1 AND published = true
	`

	post := &entities.Post{}
	err := r.db.QueryRowContext(ctx, query, slug).Scan(
		&post.ID, &post.UserID, &post.Title, &post.Content, &post.Slug,
		&post.Published, &post.CreatedAt, &post.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("post not found")
		}
		return nil, fmt.Errorf("failed to get post: %w", err)
	}

	return post, nil
}

func (r *PostRepository) GetByUserID(ctx context.Context, userID string, limit, offset int) ([]*entities.Post, error) {
	query := `
		SELECT id, user_id, title, content, slug, published, created_at, updated_at
		FROM posts 
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get user posts: %w", err)
	}
	defer rows.Close()

	return r.scanPosts(rows)
}

func (r *PostRepository) Update(ctx context.Context, post *entities.Post) error {
	query := `
		UPDATE posts 
		SET title = $2, content = $3, slug = $4, published = $5, updated_at = $6
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		post.ID, post.Title, post.Content, post.Slug, post.Published, time.Now())

	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") && strings.Contains(err.Error(), "slug") {
			return fmt.Errorf("post with slug %s already exists", post.Slug)
		}
		return fmt.Errorf("failed to update post: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("post not found")
	}

	return nil
}

func (r *PostRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM posts WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete post: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("post not found")
	}

	return nil
}

func (r *PostRepository) List(ctx context.Context, limit, offset int, publishedOnly bool) ([]*entities.Post, error) {
	query := `
		SELECT id, user_id, title, content, slug, published, created_at, updated_at
		FROM posts 
	`
	args := []interface{}{limit, offset}

	if publishedOnly {
		query += "WHERE published = true "
	}

	query += "ORDER BY created_at DESC LIMIT $1 OFFSET $2"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list posts: %w", err)
	}
	defer rows.Close()

	return r.scanPosts(rows)
}

func (r *PostRepository) Search(ctx context.Context, query string, limit, offset int, publishedOnly bool) ([]*entities.Post, error) {
	searchQuery := `
		SELECT id, user_id, title, content, slug, published, created_at, updated_at
		FROM posts 
		WHERE (title ILIKE $1 OR content ILIKE $1)
	`
	args := []interface{}{"%" + query + "%", limit, offset}

	if publishedOnly {
		searchQuery += " AND published = true"
	}

	searchQuery += " ORDER BY created_at DESC LIMIT $2 OFFSET $3"

	rows, err := r.db.QueryContext(ctx, searchQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search posts: %w", err)
	}
	defer rows.Close()

	return r.scanPosts(rows)
}

func (r *PostRepository) Exists(ctx context.Context, id string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM posts WHERE id = $1)`

	var exists bool
	err := r.db.QueryRowContext(ctx, query, id).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check post existence: %w", err)
	}

	return exists, nil
}

func (r *PostRepository) ExistsBySlug(ctx context.Context, slug string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM posts WHERE slug = $1)`

	var exists bool
	err := r.db.QueryRowContext(ctx, query, slug).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check slug existence: %w", err)
	}

	return exists, nil
}

func (r *PostRepository) GetPublishedCount(ctx context.Context) (int64, error) {
	query := `SELECT COUNT(*) FROM posts WHERE published = true`

	var count int64
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get published posts count: %w", err)
	}

	return count, nil
}

func (r *PostRepository) GetUserPostsCount(ctx context.Context, userID string) (int64, error) {
	query := `SELECT COUNT(*) FROM posts WHERE user_id = $1`

	var count int64
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get user posts count: %w", err)
	}

	return count, nil
}

func (r *PostRepository) scanPosts(rows *sql.Rows) ([]*entities.Post, error) {
	var posts []*entities.Post

	for rows.Next() {
		post := &entities.Post{}
		err := rows.Scan(
			&post.ID, &post.UserID, &post.Title, &post.Content, &post.Slug,
			&post.Published, &post.CreatedAt, &post.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan post: %w", err)
		}
		posts = append(posts, post)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration: %w", err)
	}

	return posts, nil
}
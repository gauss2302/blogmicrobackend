package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"post-service/internal/domain/entities"
	"post-service/internal/domain/repositories"
	"time"

	"github.com/lib/pq"
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
		if isSlugConflict(err) {
			return fmt.Errorf("%w: %s", repositories.ErrSlugTaken, post.Slug)
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
			return nil, repositories.ErrPostNotFound
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
			return nil, repositories.ErrPostNotFound
		}
		return nil, fmt.Errorf("failed to get post: %w", err)
	}

	return post, nil
}

func (r *PostRepository) GetByUserID(ctx context.Context, userID string, limit, offset int, includeDrafts bool) ([]*entities.Post, int, error) {
	where := ` WHERE user_id = $1 AND (published OR $2)`
	return r.page(ctx, where, `ORDER BY created_at DESC`, limit, offset, userID, includeDrafts)
}

func (r *PostRepository) Update(ctx context.Context, id string, apply func(post *entities.Post) error) (*entities.Post, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to update post: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // a no-op after Commit

	// The row lock makes a concurrent update wait and then start from this
	// one's result, so neither overwrites the other's fields.
	post := &entities.Post{}
	err = tx.QueryRowContext(ctx, `
		SELECT id, user_id, title, content, slug, published, created_at, updated_at
		FROM posts WHERE id = $1 FOR UPDATE`, id).Scan(
		&post.ID, &post.UserID, &post.Title, &post.Content, &post.Slug,
		&post.Published, &post.CreatedAt, &post.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, repositories.ErrPostNotFound
		}
		return nil, fmt.Errorf("failed to get post: %w", err)
	}

	if err := apply(post); err != nil {
		return nil, err
	}

	// updated_at is set by the update_posts_updated_at trigger.
	err = tx.QueryRowContext(ctx, `
		UPDATE posts SET title = $2, content = $3, slug = $4, published = $5
		WHERE id = $1
		RETURNING updated_at`,
		post.ID, post.Title, post.Content, post.Slug, post.Published).Scan(&post.UpdatedAt)
	if err != nil {
		if isSlugConflict(err) {
			return nil, fmt.Errorf("%w: %s", repositories.ErrSlugTaken, post.Slug)
		}
		return nil, fmt.Errorf("failed to update post: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to update post: %w", err)
	}
	return post, nil
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
		return repositories.ErrPostNotFound
	}

	return nil
}

func (r *PostRepository) List(ctx context.Context, limit, offset int, publishedOnly bool) ([]*entities.Post, int, error) {
	where := ` WHERE (published OR NOT $1)`
	return r.page(ctx, where, `ORDER BY created_at DESC`, limit, offset, publishedOnly)
}

func (r *PostRepository) Search(ctx context.Context, query string, limit, offset int, publishedOnly bool) ([]*entities.Post, int, error) {
	where := ` WHERE to_tsvector('english', title || ' ' || content) @@ plainto_tsquery('english', $1)
		AND (published OR NOT $2)`
	order := `ORDER BY ts_rank(
			to_tsvector('english', title || ' ' || content),
			plainto_tsquery('english', $1)
		) DESC, created_at DESC`
	return r.page(ctx, where, order, limit, offset, query, publishedOnly)
}

// page returns one page of the posts matching where, and how many match in
// all. Both queries share where and its args, so the total cannot drift from
// the filter; a separate COUNT stays right even past the last page.
func (r *PostRepository) page(ctx context.Context, where, order string, limit, offset int, args ...interface{}) ([]*entities.Post, int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM posts`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count posts: %w", err)
	}

	n := len(args)
	query := `SELECT id, user_id, title, content, slug, published, created_at, updated_at FROM posts` +
		where + " " + order + fmt.Sprintf(" LIMIT $%d OFFSET $%d", n+1, n+2)
	rows, err := r.db.QueryContext(ctx, query, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list posts: %w", err)
	}
	defer rows.Close()

	posts, err := r.scanPosts(rows)
	if err != nil {
		return nil, 0, err
	}
	return posts, total, nil
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

func (r *PostRepository) UnpublishByUserID(ctx context.Context, userID string) (int64, error) {
	// Only published rows are written: a retry changes nothing. The outbox
	// trigger queues each hidden post for removal from search in this statement.
	res, err := r.db.ExecContext(ctx, `UPDATE posts SET published = false WHERE user_id = $1 AND published`, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to unpublish user posts: %w", err)
	}
	return res.RowsAffected()
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

// isSlugConflict reports whether err is a unique violation of posts.slug.
// The constraint name comes from `slug ... UNIQUE` in migrations.go.
func isSlugConflict(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505" && pqErr.Constraint == "posts_slug_key"
}

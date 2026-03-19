package postgres

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"user-service/internal/domain/entities"
)

type FollowRepository struct {
	db *sql.DB
}

func NewFollowRepository(db *sql.DB) *FollowRepository {
	return &FollowRepository{db: db}
}

func (r *FollowRepository) Create(ctx context.Context, followerID, followeeID string) error {
	query := `INSERT INTO follows (follower_id, followee_id) VALUES ($1, $2) ON CONFLICT (follower_id, followee_id) DO NOTHING`
	_, err := r.db.ExecContext(ctx, query, followerID, followeeID)
	if err != nil {
		return fmt.Errorf("follow create: %w", err)
	}
	return nil
}

func (r *FollowRepository) Delete(ctx context.Context, followerID, followeeID string) error {
	query := `DELETE FROM follows WHERE follower_id = $1 AND followee_id = $2`
	_, err := r.db.ExecContext(ctx, query, followerID, followeeID)
	if err != nil {
		return fmt.Errorf("follow delete: %w", err)
	}
	return nil
}

func (r *FollowRepository) Exists(ctx context.Context, followerID, followeeID string) (bool, error) {
	query := `SELECT 1 FROM follows WHERE follower_id = $1 AND followee_id = $2 LIMIT 1`
	var one int
	err := r.db.QueryRowContext(ctx, query, followerID, followeeID).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func decodeCursor(c string) int {
	if c == "" {
		return 0
	}
	b, err := base64.StdEncoding.DecodeString(c)
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(string(b))
	if n < 0 {
		return 0
	}
	return n
}

func encodeCursor(offset int) string {
	if offset <= 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString([]byte(strconv.Itoa(offset)))
}

func (r *FollowRepository) GetFollowers(ctx context.Context, userID string, limit int, cursor string) ([]*entities.User, string, error) {
	offset := decodeCursor(cursor)
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	query := `
		SELECT u.id, u.email, u.name, u.picture, COALESCE(u.password_hash, ''), u.bio, u.location, u.website, u.is_active, u.created_at, u.updated_at
		FROM users u
		INNER JOIN follows f ON f.follower_id = u.id
		WHERE f.followee_id = $1 AND u.is_active = true
		ORDER BY f.created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, query, userID, limit+1, offset)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	var users []*entities.User
	for rows.Next() {
		u := &entities.User{}
		err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Picture, &u.PasswordHash, &u.Bio, &u.Location, &u.Website, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
		if err != nil {
			return nil, "", err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	nextCursor := ""
	if len(users) > limit {
		users = users[:limit]
		nextCursor = encodeCursor(offset + limit)
	}
	return users, nextCursor, nil
}

func (r *FollowRepository) GetFollowing(ctx context.Context, userID string, limit int, cursor string) ([]*entities.User, string, error) {
	offset := decodeCursor(cursor)
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	query := `
		SELECT u.id, u.email, u.name, u.picture, COALESCE(u.password_hash, ''), u.bio, u.location, u.website, u.is_active, u.created_at, u.updated_at
		FROM users u
		INNER JOIN follows f ON f.followee_id = u.id
		WHERE f.follower_id = $1 AND u.is_active = true
		ORDER BY f.created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, query, userID, limit+1, offset)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	var users []*entities.User
	for rows.Next() {
		u := &entities.User{}
		err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Picture, &u.PasswordHash, &u.Bio, &u.Location, &u.Website, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
		if err != nil {
			return nil, "", err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	nextCursor := ""
	if len(users) > limit {
		users = users[:limit]
		nextCursor = encodeCursor(offset + limit)
	}
	return users, nextCursor, nil
}

func (r *FollowRepository) AreFollowed(ctx context.Context, followerID string, followeeIDs []string) ([]string, error) {
	if len(followeeIDs) == 0 {
		return nil, nil
	}
	placeholders := make([]string, 0, len(followeeIDs))
	args := []interface{}{followerID}
	for i, id := range followeeIDs {
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+2))
		args = append(args, id)
	}
	query := fmt.Sprintf(
		`SELECT followee_id FROM follows WHERE follower_id = $1 AND followee_id IN (%s)`,
		strings.Join(placeholders, ","),
	)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// Package redis reads the auth-service token blacklist. It does not own the
// keys; logout and refresh write auth:blacklist:<token> in auth-service.
package redis

import (
	"context"
	"fmt"

	"github.com/go-redis/redis/v8"
)

type RevocationChecker struct {
	client *redis.Client
}

func NewRevocationChecker(addr, password string, db int) *RevocationChecker {
	return &RevocationChecker{client: redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})}
}

func (c *RevocationChecker) Revoked(ctx context.Context, token string) (bool, error) {
	n, err := c.client.Exists(ctx, "auth:blacklist:"+token).Result()
	if err != nil {
		return false, fmt.Errorf("revocation check: %w", err)
	}
	return n > 0, nil
}

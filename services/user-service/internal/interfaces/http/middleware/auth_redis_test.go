package middleware

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"user-service/internal/infrastructure/redis"
	"user-service/pkg/auth"

	goredis "github.com/go-redis/redis/v8"
)

// The key and value auth-service DeleteUserTokens writes for every access
// token of a user on logout, account deletion and refresh reuse.
func TestAuthMiddlewareRejectsTokenRevokedInRedis(t *testing.T) {
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("TEST_REDIS_ADDR not set")
	}
	ctx := context.Background()
	client := goredis.NewClient(&goredis.Options{Addr: addr})
	defer client.Close()

	validator := auth.NewValidator(testSecret)
	revoked := signToken(t, testSecret, "user-1", "access", time.Hour)
	live := signToken(t, testSecret, "user-2", "access", time.Hour)
	if err := client.Set(ctx, "auth:blacklist:"+revoked, "revoked", time.Minute).Err(); err != nil {
		t.Fatalf("SET: %v", err)
	}
	t.Cleanup(func() { client.Del(ctx, "auth:blacklist:"+revoked) })

	checker := redis.NewRevocationChecker(addr, "", 0)
	if status, actor := runRequestWithRevocations("private_network", validator, checker, map[string]string{"Authorization": "Bearer " + revoked}); status != http.StatusForbidden || actor != "" {
		t.Fatalf("revoked token: status %d actor %q, want %d and no handler", status, actor, http.StatusForbidden)
	}
	if status, actor := runRequestWithRevocations("private_network", validator, checker, map[string]string{"Authorization": "Bearer " + live}); status != http.StatusOK || actor != "user-2" {
		t.Fatalf("live token: status %d actor %q, want 200 user-2", status, actor)
	}
}

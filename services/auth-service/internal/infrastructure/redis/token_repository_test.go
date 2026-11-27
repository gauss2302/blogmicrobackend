package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"auth-service/internal/config"
	"auth-service/internal/domain/entities"
)

const refreshTTL = time.Hour

// newTestRepo connects to the Redis at TEST_REDIS_ADDR. Keys use random
// tokens and user ids, so the test never flushes or collides with other data.
func newTestRepo(t *testing.T) *TokenRepository {
	t.Helper()
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("TEST_REDIS_ADDR not set")
	}
	repo := NewTokenRepository(config.RedisConfig{URL: addr}, refreshTTL)
	t.Cleanup(func() { _ = repo.client.Close() })
	if err := repo.client.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("ping %s: %v", addr, err)
	}
	return repo
}

func randomID(t *testing.T) string {
	t.Helper()
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

func assertIndexTTL(t *testing.T, repo *TokenRepository, userID, when string) {
	t.Helper()
	ttl, err := repo.client.PTTL(context.Background(), repo.userTokenIndexKey(userID)).Result()
	if err != nil {
		t.Fatalf("PTTL: %v", err)
	}
	if ttl <= 0 || ttl > refreshTTL {
		t.Fatalf("index TTL after %s = %s, want in (0, %s]", when, ttl, refreshTTL)
	}
}

func TestUserTokenIndexExpires(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	user := "user-" + randomID(t)
	data := &entities.StoredToken{UserID: user}

	if err := repo.StoreAccessToken(ctx, "a-"+randomID(t), data, time.Minute); err != nil {
		t.Fatalf("StoreAccessToken: %v", err)
	}
	assertIndexTTL(t, repo, user, "storing an access token")

	old := "r-" + randomID(t)
	if err := repo.StoreRefreshToken(ctx, old, data, refreshTTL); err != nil {
		t.Fatalf("StoreRefreshToken: %v", err)
	}
	// Drop the TTL so the check below proves rotation sets it again.
	repo.client.Persist(ctx, repo.userTokenIndexKey(user))

	if err := repo.RotateRefreshToken(ctx, old, "r-"+randomID(t), data, refreshTTL); err != nil {
		t.Fatalf("RotateRefreshToken: %v", err)
	}
	assertIndexTTL(t, repo, user, "rotation")
}

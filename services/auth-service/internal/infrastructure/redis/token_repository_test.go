package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"auth-service/internal/config"
	"auth-service/internal/domain/entities"
	"auth-service/internal/domain/repositories"
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

func TestRotateRefreshTokenRetryIsNotReuse(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	user := "user-" + randomID(t)
	data := &entities.StoredToken{UserID: user}
	index := repo.userTokenIndexKey(user)
	old, next := "r-"+randomID(t), "r-"+randomID(t)

	if err := repo.StoreRefreshToken(ctx, old, data, refreshTTL); err != nil {
		t.Fatalf("StoreRefreshToken: %v", err)
	}
	if err := repo.RotateRefreshToken(ctx, old, next, data, refreshTTL); err != nil {
		t.Fatalf("rotate: %v", err)
	}

	// The client's retry after a read timeout: same old and new token.
	if err := repo.RotateRefreshToken(ctx, old, next, data, refreshTTL); err != nil {
		t.Fatalf("retried rotate: %v, want success", err)
	}
	if n, _ := repo.client.Exists(ctx, repo.refreshTokenKey(next)).Result(); n != 1 {
		t.Fatal("new refresh token missing after the retry")
	}
	if ok, _ := repo.client.SIsMember(ctx, index, repo.refreshTokenKey(next)).Result(); !ok {
		t.Fatal("new refresh token dropped from the user index after the retry")
	}

	// Spending the old token again for a different new one is reuse.
	if err := repo.RotateRefreshToken(ctx, old, "r-"+randomID(t), data, refreshTTL); err != repositories.ErrRefreshTokenMissing {
		t.Fatalf("reuse: err = %v, want ErrRefreshTokenMissing", err)
	}
	if blacklisted, _ := repo.IsTokenBlacklisted(ctx, old); !blacklisted {
		t.Fatal("rotated token is not blacklisted, so the service cannot tell reuse from an unknown token")
	}
}

func TestDeleteUserTokensBlacklistsAccessTokens(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	user, other := "user-"+randomID(t), "user-"+randomID(t)
	data := &entities.StoredToken{UserID: user}
	a1, a2, r1 := "a-"+randomID(t), "a-"+randomID(t), "r-"+randomID(t)
	otherAccess := "a-" + randomID(t)

	for _, a := range []string{a1, a2} {
		if err := repo.StoreAccessToken(ctx, a, data, time.Minute); err != nil {
			t.Fatalf("StoreAccessToken: %v", err)
		}
	}
	if err := repo.StoreRefreshToken(ctx, r1, data, refreshTTL); err != nil {
		t.Fatalf("StoreRefreshToken: %v", err)
	}
	if err := repo.StoreAccessToken(ctx, otherAccess, &entities.StoredToken{UserID: other}, time.Minute); err != nil {
		t.Fatalf("StoreAccessToken other: %v", err)
	}
	// An access key that expired after it was indexed must not fail the call.
	gone := "a-" + randomID(t)
	repo.client.SAdd(ctx, repo.userTokenIndexKey(user), repo.accessTokenKey(gone))

	if err := repo.DeleteUserTokens(ctx, user); err != nil {
		t.Fatalf("DeleteUserTokens: %v", err)
	}

	for _, a := range []string{a1, a2} {
		ttl, err := repo.client.PTTL(ctx, repo.blacklistKey(a)).Result()
		if err != nil || ttl <= 0 || ttl > time.Minute {
			t.Fatalf("blacklist TTL for %s = %s (err %v), want in (0, 1m]", a, ttl, err)
		}
		if n, _ := repo.client.Exists(ctx, repo.accessTokenKey(a)).Result(); n != 0 {
			t.Fatalf("access key %s still stored", a)
		}
	}
	for _, key := range []string{repo.refreshTokenKey(r1), repo.userTokenIndexKey(user), repo.blacklistKey(gone)} {
		if n, _ := repo.client.Exists(ctx, key).Result(); n != 0 {
			t.Fatalf("%s exists after DeleteUserTokens", key)
		}
	}
	if blacklisted, _ := repo.IsTokenBlacklisted(ctx, otherAccess); blacklisted {
		t.Fatal("another user's access token was blacklisted")
	}
	if n, _ := repo.client.Exists(ctx, repo.accessTokenKey(otherAccess)).Result(); n != 1 {
		t.Fatal("another user's access token was deleted")
	}
}

// Without the SCAN fallback an unindexed token is left alone; DeleteUserTokens
// relies on the index alone.
func TestDeleteUserTokensDoesNotScanTheKeyspace(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	user := "user-" + randomID(t)
	stray := repo.accessTokenKey("a-" + randomID(t))
	if err := repo.client.Set(ctx, stray, `{"user_id":"`+user+`"}`, time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { repo.client.Del(ctx, stray) })

	if err := repo.DeleteUserTokens(ctx, user); err != nil {
		t.Fatalf("DeleteUserTokens: %v", err)
	}
	if n, _ := repo.client.Exists(ctx, stray).Result(); n != 1 {
		t.Fatal("an unindexed key was deleted: the keyspace was scanned")
	}
}

// The index is all DeleteUserTokens has, so it must outlive every token in
// it: writing a short-lived access token after a refresh token must not
// shorten it below the refresh token's TTL.
func TestUserTokenIndexOutlivesItsRefreshToken(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	user := "user-" + randomID(t)
	data := &entities.StoredToken{UserID: user}
	refresh := "r-" + randomID(t)

	if err := repo.StoreRefreshToken(ctx, refresh, data, refreshTTL); err != nil {
		t.Fatal(err)
	}
	if err := repo.StoreAccessToken(ctx, "a-"+randomID(t), data, time.Minute); err != nil {
		t.Fatal(err)
	}
	indexTTL, _ := repo.client.PTTL(ctx, repo.userTokenIndexKey(user)).Result()
	refreshLeft, _ := repo.client.PTTL(ctx, repo.refreshTokenKey(refresh)).Result()
	if refreshLeft <= 0 || indexTTL < refreshLeft {
		t.Fatalf("index TTL %s < refresh token TTL %s", indexTTL, refreshLeft)
	}
}

func TestLoginFailureCounterWindow(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	email := "User-" + randomID(t) + "@Example.com "
	key := repo.loginFailKey(email)
	t.Cleanup(func() { repo.client.Del(ctx, key) })

	if strings.Contains(key, "example") || key != repo.loginFailKey(strings.ToLower(strings.TrimSpace(email))) {
		t.Fatalf("key %q must be a hash of the normalised email", key)
	}
	if err := repo.RecordLoginFailure(ctx, email, time.Minute); err != nil {
		t.Fatal(err)
	}
	repo.client.PExpire(ctx, key, 30*time.Second) // a later failure must not restart the window
	if err := repo.RecordLoginFailure(ctx, email, time.Minute); err != nil {
		t.Fatal(err)
	}
	if n, err := repo.LoginFailures(ctx, email); err != nil || n != 2 {
		t.Fatalf("failures %d err %v, want 2", n, err)
	}
	if ttl, _ := repo.client.PTTL(ctx, key).Result(); ttl <= 0 || ttl > 30*time.Second {
		t.Fatalf("window TTL %s, want kept from the first failure (<= 30s)", ttl)
	}
	if err := repo.ClearLoginFailures(ctx, email); err != nil {
		t.Fatal(err)
	}
	if n, err := repo.LoginFailures(ctx, email); err != nil || n != 0 {
		t.Fatalf("after clear: failures %d err %v, want 0", n, err)
	}
}

func TestGetTokenDataTellsMissingFromFailure(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	if _, err := repo.GetTokenData(ctx, "missing-"+randomID(t)); !errors.Is(err, repositories.ErrTokenNotFound) {
		t.Fatalf("missing token: err %v, want ErrTokenNotFound", err)
	}

	broken := NewTokenRepository(config.RedisConfig{URL: "127.0.0.1:1"}, refreshTTL)
	defer broken.client.Close()
	if _, err := broken.GetTokenData(ctx, "any"); err == nil || errors.Is(err, repositories.ErrTokenNotFound) {
		t.Fatalf("unreachable Redis: err %v, want a store error, not ErrTokenNotFound", err)
	}
}

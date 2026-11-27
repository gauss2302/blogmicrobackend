package redis

import (
	"auth-service/internal/config"
	"auth-service/internal/domain/entities"
	"auth-service/internal/domain/repositories"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

type TokenRepository struct {
	client *redis.Client
	// indexTTL is the refresh-token TTL. Every write to a user's token index
	// resets the index to it, so the index outlives the newest refresh token
	// and then expires instead of living forever.
	indexTTL time.Duration
}

func NewTokenRepository(cfg config.RedisConfig, indexTTL time.Duration) *TokenRepository {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.URL,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	return &TokenRepository{client: client, indexTTL: indexTTL}
}

// Auth code management
func (r *TokenRepository) StoreAuthCode(ctx context.Context, authCode string, payload *entities.AuthCodePayload, ttl time.Duration) error {
	key := r.authCodeKey(authCode)

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal auth code payload: %w", err)
	}

	return r.client.Set(ctx, key, jsonData, ttl).Err()
}

func (r *TokenRepository) GetAndDeleteAuthCode(ctx context.Context, authCode string) (*entities.AuthCodePayload, error) {
	key := r.authCodeKey(authCode)

	data, err := r.getAndDelete(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve auth code payload: %w", err)
	}

	var payload entities.AuthCodePayload
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal auth code payload: %w", err)
	}

	if payload.User == nil {
		return nil, fmt.Errorf("auth code payload missing user")
	}

	return &payload, nil
}

// OAuth state management (CRITICAL for security)
func (r *TokenRepository) StoreState(ctx context.Context, state string, payload *entities.OAuthState, ttl time.Duration) error {
	key := r.stateKey(state)

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal oauth state: %w", err)
	}

	return r.client.Set(ctx, key, jsonData, ttl).Err()
}

func (r *TokenRepository) GetAndDeleteState(ctx context.Context, state string) (*entities.OAuthState, error) {
	key := r.stateKey(state)

	data, err := r.getAndDelete(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve oauth state: %w", err)
	}

	var oauthState entities.OAuthState
	if err := json.Unmarshal([]byte(data), &oauthState); err != nil {
		return nil, fmt.Errorf("failed to unmarshal oauth state: %w", err)
	}

	if oauthState.State == "" {
		return nil, fmt.Errorf("oauth state payload is invalid")
	}

	return &oauthState, nil
}

// Token management
func (r *TokenRepository) StoreAccessToken(ctx context.Context, token string, data *entities.StoredToken, ttl time.Duration) error {
	key := r.accessTokenKey(token)
	return r.storeToken(ctx, key, data, ttl)
}

func (r *TokenRepository) StoreRefreshToken(ctx context.Context, token string, data *entities.StoredToken, ttl time.Duration) error {
	key := r.refreshTokenKey(token)
	return r.storeToken(ctx, key, data, ttl)
}

// GetTokenData returns repositories.ErrTokenNotFound when neither an access
// nor a refresh record exists; a Redis failure comes back as itself.
func (r *TokenRepository) GetTokenData(ctx context.Context, token string) (*entities.StoredToken, error) {
	data, err := r.getToken(ctx, r.accessTokenKey(token))
	if !errors.Is(err, redis.Nil) {
		return data, err
	}
	data, err = r.getToken(ctx, r.refreshTokenKey(token))
	if errors.Is(err, redis.Nil) {
		return nil, repositories.ErrTokenNotFound
	}
	return data, err
}

func (r *TokenRepository) DeleteToken(ctx context.Context, token string) error {
	keys := []string{
		r.accessTokenKey(token),
		r.refreshTokenKey(token),
	}

	pipe := r.client.Pipeline()
	for _, key := range keys {
		pipe.Del(ctx, key)
	}

	if tokenData, err := r.GetTokenData(ctx, token); err == nil && tokenData != nil && tokenData.UserID != "" {
		members := make([]interface{}, 0, len(keys))
		for _, key := range keys {
			members = append(members, key)
		}
		pipe.SRem(ctx, r.userTokenIndexKey(tokenData.UserID), members...)
	}

	_, err := pipe.Exec(ctx)
	return err
}

func (r *TokenRepository) DeleteUserTokens(ctx context.Context, userID string) error {
	keys, err := r.client.SMembers(ctx, r.userTokenIndexKey(userID)).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("failed to get user token index: %w", err)
	}

	// No fallback SCAN of the keyspace: logout is public, so anyone with one
	// valid token could trigger it. Every token is indexed when stored, and the
	// index outlives them all (storeToken), so an empty index means none live.
	if len(keys) == 0 {
		return nil
	}

	if err := revokeUserTokensScript.Run(ctx, r.client, append(keys, r.userTokenIndexKey(userID))).Err(); err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("failed to delete user tokens: %w", err)
	}
	return nil
}

// revokeUserTokensScript deletes a user's token keys and the index (the last
// key). Each access token still in the store is blacklisted for the rest of
// its TTL first: the internal HTTP APIs check only auth:blacklist:<token>, not
// the store. A key that already expired is skipped; that token is dead anyway.
var revokeUserTokensScript = redis.NewScript(`
for i = 1, #KEYS - 1 do
  local k = KEYS[i]
  if string.sub(k, 1, 12) == 'auth:access:' then
    local ttl = redis.call('PTTL', k)
    if ttl > 0 then
      redis.call('SET', 'auth:blacklist:' .. string.sub(k, 13), 'revoked', 'PX', ttl)
    elseif ttl == -1 then
      redis.call('SET', 'auth:blacklist:' .. string.sub(k, 13), 'revoked')
    end
  end
  redis.call('DEL', k)
end
redis.call('DEL', KEYS[#KEYS])
`)

// rotateRefreshScript claims the old refresh token, stores the replacement, and
// blacklists the old value in one Redis execution. A second caller finds the
// key gone and cannot mint another session from the same token. A failed
// script writes nothing, so a blacklist error does not issue new tokens.
//
// The client retries EVALSHA after a read timeout, so the script may run twice
// with the same arguments. Old key gone but this call's new key present means
// the first run already rotated: report success, not reuse. A reuse always
// brings a different new token (each carries a random jti).
var rotateRefreshScript = redis.NewScript(`
local old = redis.call('GET', KEYS[1])
if not old then
  if redis.call('EXISTS', KEYS[2]) == 1 then
    return 'ok'
  end
  return ''
end
redis.call('DEL', KEYS[1])
redis.call('SET', KEYS[2], ARGV[1], 'PX', ARGV[2])
redis.call('SET', KEYS[3], 'rotated', 'PX', ARGV[2])
if KEYS[4] ~= '' then
  redis.call('SREM', KEYS[4], KEYS[1])
  redis.call('SADD', KEYS[4], KEYS[2])
  redis.call('PEXPIRE', KEYS[4], ARGV[2])
end
return 'ok'
`)

// RotateRefreshToken atomically swaps oldToken for newToken. It returns
// repositories.ErrRefreshTokenMissing when oldToken is already gone.
func (r *TokenRepository) RotateRefreshToken(ctx context.Context, oldToken, newToken string, data *entities.StoredToken, ttl time.Duration) error {
	if data == nil || data.UserID == "" {
		return fmt.Errorf("refresh rotation requires a user id")
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal token data: %w", err)
	}
	ttlMs := ttl.Milliseconds()
	if ttlMs < 1 {
		ttlMs = 1
	}

	res, err := rotateRefreshScript.Run(ctx, r.client, []string{
		r.refreshTokenKey(oldToken),
		r.refreshTokenKey(newToken),
		r.blacklistKey(oldToken),
		r.userTokenIndexKey(data.UserID),
	}, string(jsonData), ttlMs).Result()
	if err != nil {
		return fmt.Errorf("rotate refresh token: %w", err)
	}
	status, _ := res.(string)
	if status == "" {
		return repositories.ErrRefreshTokenMissing
	}
	return nil
}

// Blacklist management
func (r *TokenRepository) IsTokenBlacklisted(ctx context.Context, token string) (bool, error) {
	key := r.blacklistKey(token)
	exists, err := r.client.Exists(ctx, key).Result()
	return exists > 0, err
}

func (r *TokenRepository) BlacklistToken(ctx context.Context, token string, ttl time.Duration) error {
	key := r.blacklistKey(token)
	return r.client.Set(ctx, key, "blacklisted", ttl).Err()
}

// Security audit logging (optional but recommended)
func (r *TokenRepository) LogAuthAttempt(ctx context.Context, userID, ip, userAgent string, success bool) error {
	logEntry := map[string]interface{}{
		"user_id":    userID,
		"ip":         ip,
		"user_agent": userAgent,
		"success":    success,
		"timestamp":  time.Now().Unix(),
	}

	jsonData, err := json.Marshal(logEntry)
	if err != nil {
		return fmt.Errorf("failed to marshal auth log: %w", err)
	}

	// Store with a key that allows for easy querying
	key := fmt.Sprintf("auth:log:%s:%d", userID, time.Now().Unix())

	// Keep auth logs for 30 days
	return r.client.Set(ctx, key, jsonData, 30*24*time.Hour).Err()
}

// recordLoginFailureScript counts a failure and starts the window on the first.
var recordLoginFailureScript = redis.NewScript(`
if redis.call('INCR', KEYS[1]) == 1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
return 1
`)

func (r *TokenRepository) LoginFailures(ctx context.Context, email string) (int64, error) {
	n, err := r.client.Get(ctx, r.loginFailKey(email)).Int64()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	return n, err
}

func (r *TokenRepository) RecordLoginFailure(ctx context.Context, email string, window time.Duration) error {
	err := recordLoginFailureScript.Run(ctx, r.client, []string{r.loginFailKey(email)}, window.Milliseconds()).Err()
	if errors.Is(err, redis.Nil) {
		return nil
	}
	return err
}

func (r *TokenRepository) ClearLoginFailures(ctx context.Context, email string) error {
	return r.client.Del(ctx, r.loginFailKey(email)).Err()
}

// loginFailKey hashes the email so Redis holds no addresses.
func (r *TokenRepository) loginFailKey(email string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(email))))
	return "auth:login_fail:" + hex.EncodeToString(sum[:])
}

// Private helper methods
func (r *TokenRepository) storeToken(ctx context.Context, key string, data *entities.StoredToken, ttl time.Duration) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	pipe := r.client.Pipeline()
	pipe.Set(ctx, key, jsonData, ttl)
	if data != nil && data.UserID != "" {
		indexKey := r.userTokenIndexKey(data.UserID)
		pipe.SAdd(ctx, indexKey, key)
		if r.indexTTL > 0 { // EXPIRE 0 would delete the index
			pipe.Expire(ctx, indexKey, r.indexTTL)
		}
	}

	_, err = pipe.Exec(ctx)
	return err
}

func (r *TokenRepository) getToken(ctx context.Context, key string) (*entities.StoredToken, error) {
	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var storedToken entities.StoredToken
	if err := json.Unmarshal([]byte(data), &storedToken); err != nil {
		return nil, err
	}

	return &storedToken, nil
}

// Key generation methods
func (r *TokenRepository) accessTokenKey(token string) string {
	return fmt.Sprintf("auth:access:%s", token)
}

func (r *TokenRepository) refreshTokenKey(token string) string {
	return fmt.Sprintf("auth:refresh:%s", token)
}

func (r *TokenRepository) blacklistKey(token string) string {
	return fmt.Sprintf("auth:blacklist:%s", token)
}

func (r *TokenRepository) authCodeKey(authCode string) string {
	return fmt.Sprintf("auth:code:%s", authCode)
}

func (r *TokenRepository) stateKey(state string) string {
	return fmt.Sprintf("auth:state:%s", state)
}

func (r *TokenRepository) userTokenIndexKey(userID string) string {
	return fmt.Sprintf("auth:user_tokens:%s", userID)
}

func (r *TokenRepository) getAndDelete(ctx context.Context, key string) (string, error) {
	data, err := r.client.Do(ctx, "GETDEL", key).Text()
	if errors.Is(err, redis.Nil) {
		return "", fmt.Errorf("key not found or expired")
	}
	if err != nil {
		return "", err
	}
	return data, nil
}

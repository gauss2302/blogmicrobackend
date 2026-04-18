package redis

import (
	"auth-service/internal/config"
	"auth-service/internal/domain/entities"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

type TokenRepository struct {
	client *redis.Client
}

func NewTokenRepository(cfg config.RedisConfig) *TokenRepository {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.URL,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	return &TokenRepository{client: client}
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

func (r *TokenRepository) GetTokenData(ctx context.Context, token string) (*entities.StoredToken, error) {
	// Try access token first
	key := r.accessTokenKey(token)
	data, err := r.getToken(ctx, key)
	if err == nil {
		return data, nil
	}

	// Try refresh token
	key = r.refreshTokenKey(token)
	return r.getToken(ctx, key)
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

	// Backward compatible fallback for tokens stored before index support.
	if len(keys) == 0 {
		keys, err = r.scanUserTokenKeys(ctx, userID)
		if err != nil {
			return fmt.Errorf("failed to scan user tokens: %w", err)
		}
	}
	if len(keys) == 0 {
		return nil
	}

	indexKey := r.userTokenIndexKey(userID)
	pipe := r.client.Pipeline()
	for _, key := range keys {
		pipe.Del(ctx, key)
	}
	pipe.Del(ctx, indexKey)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete user tokens: %w", err)
	}
	return nil
}

// Token rotation (security best practice)
func (r *TokenRepository) RotateRefreshToken(ctx context.Context, oldToken, newToken string, data *entities.StoredToken, ttl time.Duration) error {
	pipe := r.client.Pipeline()

	// Delete old token
	oldKey := r.refreshTokenKey(oldToken)
	pipe.Del(ctx, oldKey)

	// Store new token
	newKey := r.refreshTokenKey(newToken)
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal token data: %w", err)
	}
	pipe.Set(ctx, newKey, jsonData, ttl)
	if data != nil && data.UserID != "" {
		indexKey := r.userTokenIndexKey(data.UserID)
		pipe.SRem(ctx, indexKey, oldKey)
		pipe.SAdd(ctx, indexKey, newKey)
	}

	// Blacklist old token to prevent reuse
	blacklistKey := r.blacklistKey(oldToken)
	pipe.Set(ctx, blacklistKey, "rotated", ttl)

	_, err = pipe.Exec(ctx)
	return err
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

// Private helper methods
func (r *TokenRepository) storeToken(ctx context.Context, key string, data *entities.StoredToken, ttl time.Duration) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	pipe := r.client.Pipeline()
	pipe.Set(ctx, key, jsonData, ttl)
	if data != nil && data.UserID != "" {
		pipe.SAdd(ctx, r.userTokenIndexKey(data.UserID), key)
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

func (r *TokenRepository) scanUserTokenKeys(ctx context.Context, userID string) ([]string, error) {
	var (
		cursor uint64
		keys   []string
	)

	for {
		batch, nextCursor, err := r.client.Scan(ctx, cursor, "auth:*:*", 500).Result()
		if err != nil {
			return nil, err
		}
		for _, key := range batch {
			if !strings.HasPrefix(key, "auth:access:") && !strings.HasPrefix(key, "auth:refresh:") {
				continue
			}

			tokenData, getErr := r.client.Get(ctx, key).Result()
			if getErr != nil {
				continue
			}

			var storedToken entities.StoredToken
			if unmarshalErr := json.Unmarshal([]byte(tokenData), &storedToken); unmarshalErr != nil {
				continue
			}
			if storedToken.UserID == userID {
				keys = append(keys, key)
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return keys, nil
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

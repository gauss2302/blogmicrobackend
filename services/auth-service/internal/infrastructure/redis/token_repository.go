package redis

import (
	"auth-service/internal/config"
	"auth-service/internal/domain/entities"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
func (r *TokenRepository) StoreAuthCode(ctx context.Context, authCode string, userInfo *entities.GoogleUserInfo, ttl time.Duration) error {
	key := r.authCodeKey(authCode)

	jsonData, err := json.Marshal(userInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal user info: %w", err)
	}

	return r.client.Set(ctx, key, jsonData, ttl).Err()
}

func (r *TokenRepository) GetAndDeleteAuthCode(ctx context.Context, authCode string) (*entities.GoogleUserInfo, error) {
	key := r.authCodeKey(authCode)

	pipe := r.client.Pipeline()
	getCmd := pipe.Get(ctx, key)
	pipe.Del(ctx, key)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get auth code: %w", err)
	}

	data, err := getCmd.Result()
	if errors.Is(err, redis.Nil) {
		return nil, fmt.Errorf("auth code not found or expired")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to retrieve auth code: %w", err)
	}

	var userInfo entities.GoogleUserInfo
	if err := json.Unmarshal([]byte(data), &userInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user info: %w", err)
	}

	return &userInfo, nil
}

// OAuth state management (CRITICAL for security)
func (r *TokenRepository) StoreState(ctx context.Context, key, state string, ttl time.Duration) error {
	return r.client.Set(ctx, key, state, ttl).Err()
}

func (r *TokenRepository) GetAndDeleteState(ctx context.Context, key string) (string, error) {
	pipe := r.client.Pipeline()
	getCmd := pipe.Get(ctx, key)
	pipe.Del(ctx, key)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return "", fmt.Errorf("pipeline execution failed: %w", err)
	}

	state, err := getCmd.Result()
	if errors.Is(err, redis.Nil) {
		return "", fmt.Errorf("state not found or expired")
	}

	if err != nil {
		return "", fmt.Errorf("failed to retrieve state: %w", err)
	}

	return state, nil
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

	_, err := pipe.Exec(ctx)
	return err
}

func (r *TokenRepository) DeleteUserTokens(ctx context.Context, userID string) error {
	pattern := fmt.Sprintf("auth:*:*")
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return err
	}

	for _, key := range keys {
		data, err := r.client.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var storedToken entities.StoredToken
		if err := json.Unmarshal([]byte(data), &storedToken); err != nil {
			continue
		}

		if storedToken.UserID == userID {
			r.client.Del(ctx, key)
		}
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

	return r.client.Set(ctx, key, jsonData, ttl).Err()
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

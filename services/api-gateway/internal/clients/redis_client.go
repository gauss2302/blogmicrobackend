package clients

import (
	"context"
	"time"

	"api-gateway/internal/config"

	"github.com/go-redis/redis/v8"
)

type RedisClient struct {
	client *redis.Client
}

func NewRedisClient(cfg config.RedisConfig) *RedisClient {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.URL,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	return &RedisClient{client: client}
}

func (r *RedisClient) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

func (r *RedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return r.client.Set(ctx, key, value, expiration).Err()
}

// incrWindowScript increments a counter and gives it a TTL whenever it has
// none, in one atomic step: a key can never be left without an expiry, and a
// key left TTL-less by older code gets one on its next hit.
var incrWindowScript = redis.NewScript(`
local count = redis.call('INCR', KEYS[1])
if redis.call('PTTL', KEYS[1]) < 0 then
	redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
return {count, redis.call('PTTL', KEYS[1])}
`)

// IncrWindow increments key within a fixed window and returns the new count
// and the time left until the window resets.
func (r *RedisClient) IncrWindow(ctx context.Context, key string, window time.Duration) (int64, time.Duration, error) {
	res, err := incrWindowScript.Run(ctx, r.client, []string{key}, window.Milliseconds()).Int64Slice()
	if err != nil {
		return 0, 0, err
	}
	return res[0], time.Duration(res[1]) * time.Millisecond, nil
}

func (r *RedisClient) Exists(ctx context.Context, key string) (int64, error) {
	return r.client.Exists(ctx, key).Result()
}

func (r *RedisClient) Del(ctx context.Context, keys ...string) error {
	return r.client.Del(ctx, keys...).Err()
}

func (r *RedisClient) Close() error {
	return r.client.Close()
}

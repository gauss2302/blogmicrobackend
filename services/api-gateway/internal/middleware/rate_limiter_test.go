package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"

	"api-gateway/internal/clients"
	"api-gateway/internal/config"
)

// Needs a real Redis: TEST_REDIS_ADDR=127.0.0.1:<port> go test ./internal/middleware -run RateLimit
func TestRateLimitRedis(t *testing.T) {
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("TEST_REDIS_ADDR not set")
	}
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	rc := clients.NewRedisClient(config.RedisConfig{URL: addr})
	raw := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { rc.Close(); raw.Close() })

	// failClosed makes any Redis error a visible 503 instead of a silent
	// fallback to the in-memory limiter.
	setup := func(t *testing.T, limit int) (*gin.Engine, string) {
		prefix := fmt.Sprintf("test_rate_limit_%d", time.Now().UnixNano())
		key := prefix + ":192.0.2.1" // httptest.NewRequest's RemoteAddr
		t.Cleanup(func() { raw.Del(ctx, key) })
		r := gin.New()
		r.Use(rateLimit(rc, rateLimitOptions{enabled: true, requestsPerMin: limit, burstSize: limit, keyPrefix: prefix, failClosed: true}))
		r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })
		return r, key
	}
	serve := func(r *gin.Engine, req *http.Request) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	get := func() *http.Request { return httptest.NewRequest(http.MethodGet, "/", nil) }
	wantTTL := func(t *testing.T, key string) {
		t.Helper()
		ttl, err := raw.TTL(ctx, key).Result()
		if err != nil {
			t.Fatal(err)
		}
		if ttl <= 0 || ttl > time.Minute {
			t.Fatalf("TTL = %v, want (0, 60s]", ttl)
		}
	}

	t.Run("first request sets TTL", func(t *testing.T) {
		r, key := setup(t, 5)
		if w := serve(r, get()); w.Code != http.StatusOK {
			t.Fatalf("status = %d", w.Code)
		}
		wantTTL(t, key)
	})

	t.Run("key left without TTL gets one", func(t *testing.T) {
		r, key := setup(t, 100)
		if err := raw.Set(ctx, key, 7, 0).Err(); err != nil {
			t.Fatal(err)
		}
		serve(r, get())
		wantTTL(t, key)
	})

	t.Run("cancelled request context still sets TTL", func(t *testing.T) {
		r, key := setup(t, 5)
		reqCtx, cancel := context.WithCancel(ctx)
		cancel()
		if w := serve(r, get().WithContext(reqCtx)); w.Code != http.StatusOK {
			t.Fatalf("status = %d, body %s", w.Code, w.Body)
		}
		wantTTL(t, key)
	})

	t.Run("N pass, N+1 is rejected", func(t *testing.T) {
		const n = 3
		r, _ := setup(t, n)
		for i := 1; i <= n; i++ {
			w := serve(r, get())
			if w.Code != http.StatusOK {
				t.Fatalf("request %d: status = %d", i, w.Code)
			}
			if got, want := w.Header().Get("X-RateLimit-Remaining"), strconv.Itoa(n-i); got != want {
				t.Fatalf("request %d: remaining = %q, want %q", i, got, want)
			}
		}
		if w := serve(r, get()); w.Code != http.StatusTooManyRequests {
			t.Fatalf("request %d: status = %d, want 429", n+1, w.Code)
		}
	})
}

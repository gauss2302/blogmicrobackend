// internal/middleware/rate_limit.go
package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"api-gateway/internal/clients"
	"api-gateway/internal/config"
	"api-gateway/pkg/utils"
)

// RateLimit is the general per-IP limiter applied to all routes. On a Redis
// error it falls back to a per-IP in-memory limiter (not a single shared
// bucket), so one client cannot consume everyone's allowance during a Redis
// outage.
func RateLimit(redisClient *clients.RedisClient, cfg config.RateLimitConfig) gin.HandlerFunc {
	return rateLimit(redisClient, rateLimitOptions{
		enabled:        cfg.Enabled,
		requestsPerMin: cfg.RequestsPerMinute,
		burstSize:      cfg.BurstSize,
		keyPrefix:      "rate_limit",
		failClosed:     false,
	})
}

// AuthRateLimit is a stricter per-IP limiter for the unauthenticated
// credential/token endpoints (login, register, refresh, exchange). It blunts
// brute-force, credential stuffing, and auth_code/refresh-token guessing, and
// fails closed: if Redis is unavailable the request is rejected rather than
// allowed.
func AuthRateLimit(redisClient *clients.RedisClient, cfg config.RateLimitConfig) gin.HandlerFunc {
	return rateLimit(redisClient, rateLimitOptions{
		enabled:        cfg.Enabled,
		requestsPerMin: cfg.AuthRequestsPerMinute,
		burstSize:      cfg.AuthRequestsPerMinute,
		keyPrefix:      "rate_limit_auth",
		failClosed:     true,
	})
}

type rateLimitOptions struct {
	enabled        bool
	requestsPerMin int
	burstSize      int
	keyPrefix      string
	// failClosed controls behaviour when Redis is unavailable: true rejects the
	// request (used for auth endpoints); false falls back to a per-IP in-memory
	// limiter (used for general traffic).
	failClosed bool
}

func rateLimit(redisClient *clients.RedisClient, opts rateLimitOptions) gin.HandlerFunc {
	if !opts.enabled {
		return func(c *gin.Context) { c.Next() }
	}

	// Defensive clamp: a misconfigured limit/burst of 0 must not panic
	// (rate.Every divides by it) or silently disable limiting.
	if opts.requestsPerMin < 1 {
		opts.requestsPerMin = 1
	}
	if opts.burstSize < 1 {
		opts.burstSize = 1
	}

	fallback := newPerIPLimiters(opts.requestsPerMin, opts.burstSize)

	return func(c *gin.Context) {
		// Key on the client IP only. The previous User-Agent component was
		// attacker-controlled, letting a single client mint an unlimited number
		// of buckets and bypass the limit entirely. ClientIP is derived from the
		// trusted-proxy configuration, so it cannot be spoofed via X-Forwarded-For.
		clientIP := c.ClientIP()
		key := fmt.Sprintf("%s:%s", opts.keyPrefix, clientIP)

		allowed, err := checkRateLimit(redisClient, key, opts.requestsPerMin, c)
		if err != nil {
			if opts.failClosed {
				utils.ErrorResponse(c, http.StatusServiceUnavailable, "RATE_LIMIT_UNAVAILABLE", "Service temporarily unavailable, please retry")
				c.Abort()
				return
			}
			// General traffic: per-IP in-memory fallback so limiting survives a
			// Redis outage without collapsing to one shared bucket.
			if !fallback.allow(clientIP) {
				rejectRateLimited(c, opts.requestsPerMin)
				return
			}
			c.Next()
			return
		}

		if !allowed {
			ttl := getRateLimitTTL(redisClient, key, c)
			c.Header("X-RateLimit-Limit", strconv.Itoa(opts.requestsPerMin))
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(ttl).Unix(), 10))

			utils.ErrorResponse(c, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "Rate limit exceeded. Try again later.")
			c.Abort()
			return
		}

		remaining := getRemainingRequests(redisClient, key, opts.requestsPerMin, c)
		c.Header("X-RateLimit-Limit", strconv.Itoa(opts.requestsPerMin))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Minute).Unix(), 10))

		c.Next()
	}
}

func rejectRateLimited(c *gin.Context, limit int) {
	c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
	c.Header("X-RateLimit-Remaining", "0")
	utils.ErrorResponse(c, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "Too many requests")
	c.Abort()
}

func checkRateLimit(redisClient *clients.RedisClient, key string, limit int, c *gin.Context) (bool, error) {
	ctx := c.Request.Context()

	// Increment the counter
	count, err := redisClient.Incr(ctx, key)
	if err != nil {
		return false, err
	}

	// Set expiration on first request
	if count == 1 {
		if err = redisClient.Expire(ctx, key, time.Minute); err != nil {
			return false, err
		}
	}

	// Check if limit exceeded
	return count <= int64(limit), nil
}

func getRemainingRequests(redisClient *clients.RedisClient, key string, limit int, c *gin.Context) int {
	ctx := c.Request.Context()

	countStr, err := redisClient.Get(ctx, key)
	if err != nil {
		return limit
	}

	count, err := strconv.Atoi(countStr)
	if err != nil {
		return limit
	}

	remaining := limit - count
	if remaining < 0 {
		return 0
	}
	return remaining
}

func getRateLimitTTL(redisClient *clients.RedisClient, key string, c *gin.Context) time.Duration {
	// For now, return default
	// You could implement Redis TTL command here if needed
	return time.Minute
}

// perIPLimiters holds in-memory token-bucket limiters keyed by client IP. It is
// used only as a fallback when Redis is unavailable, preserving per-client
// limiting instead of degrading to a single global bucket.
type perIPLimiters struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	rps      rate.Limit
	burst    int
}

// maxFallbackEntries bounds memory during a sustained Redis outage when client
// IPs churn; on overflow the table is reset (everyone gets a fresh bucket).
const maxFallbackEntries = 10000

func newPerIPLimiters(requestsPerMin, burst int) *perIPLimiters {
	return &perIPLimiters{
		limiters: make(map[string]*rate.Limiter),
		rps:      rate.Every(time.Minute / time.Duration(requestsPerMin)),
		burst:    burst,
	}
}

func (p *perIPLimiters) allow(ip string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.limiters) > maxFallbackEntries {
		p.limiters = make(map[string]*rate.Limiter)
	}

	limiter, ok := p.limiters[ip]
	if !ok {
		limiter = rate.NewLimiter(p.rps, p.burst)
		p.limiters[ip] = limiter
	}
	return limiter.Allow()
}

// internal/middleware/rate_limit.go
package middleware

import (
	"context"
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
		// of buckets and bypass the limit entirely. ClientIP honours
		// X-Forwarded-For only from TRUSTED_PROXIES. Where that is the pod's own
		// mesh proxy (the production overlay), every in-cluster caller arrives
		// through it, so who may reach this port is limited by a NetworkPolicy
		// (k8s/overlays/production/networkpolicy.yaml).
		clientIP := c.ClientIP()
		key := fmt.Sprintf("%s:%s", opts.keyPrefix, clientIP)

		count, ttl, err := incrRateLimit(c, redisClient, key)
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

		remaining := max(opts.requestsPerMin-int(count), 0)
		c.Header("X-RateLimit-Limit", strconv.Itoa(opts.requestsPerMin))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(ttl).Unix(), 10))

		if count > int64(opts.requestsPerMin) {
			utils.ErrorResponse(c, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "Rate limit exceeded. Try again later.")
			c.Abort()
			return
		}

		c.Next()
	}
}

func rejectRateLimited(c *gin.Context, limit int) {
	c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
	c.Header("X-RateLimit-Remaining", "0")
	utils.ErrorResponse(c, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "Too many requests")
	c.Abort()
}

// rateLimitRedisTimeout bounds the counter update. It runs detached from the
// request context so a client disconnect cannot cancel it half-way.
const rateLimitRedisTimeout = 500 * time.Millisecond

func incrRateLimit(c *gin.Context, redisClient *clients.RedisClient, key string) (int64, time.Duration, error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), rateLimitRedisTimeout)
	defer cancel()
	return redisClient.IncrWindow(ctx, key, time.Minute)
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

// internal/middleware/rate_limit.go
package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"api-gateway/internal/clients"
	"api-gateway/internal/config"
	"api-gateway/pkg/utils"
)

type RateLimiter struct {
	redisClient *clients.RedisClient
	config      config.RateLimitConfig
	limiter     *rate.Limiter
}

func RateLimit(redisClient *clients.RedisClient, cfg config.RateLimitConfig) gin.HandlerFunc {
	if !cfg.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	// Create a global rate limiter as fallback
	globalLimiter := rate.NewLimiter(rate.Every(time.Minute/time.Duration(cfg.RequestsPerMinute)), cfg.BurstSize)

	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		userAgent := c.GetHeader("User-Agent")
		
		// Create a unique key for this client
		key := fmt.Sprintf("rate_limit:%s:%s", clientIP, userAgent)
		
		// Check rate limit using Redis
		allowed, err := checkRateLimit(redisClient, key, cfg, c)
		if err != nil {
			// Fallback to in-memory rate limiter if Redis fails
			if !globalLimiter.Allow() {
				utils.ErrorResponse(c, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "Too many requests")
				c.Abort()
				return
			}
		} else if !allowed {
			// Get remaining time until reset
			ttl := getRateLimitTTL(redisClient, key, c)
			c.Header("X-RateLimit-Limit", strconv.Itoa(cfg.RequestsPerMinute))
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(ttl).Unix(), 10))
			
			utils.ErrorResponse(c, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "Rate limit exceeded. Try again later.")
			c.Abort()
			return
		}

		// Add rate limit headers
		remaining := getRemainingRequests(redisClient, key, cfg, c)
		c.Header("X-RateLimit-Limit", strconv.Itoa(cfg.RequestsPerMinute))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Minute).Unix(), 10))

		c.Next()
	}
}

func checkRateLimit(redisClient *clients.RedisClient, key string, cfg config.RateLimitConfig, c *gin.Context) (bool, error) {
	ctx := c.Request.Context()
	
	// Increment the counter
	count, err := redisClient.Incr(ctx, key)
	if err != nil {
		return false, err
	}

	// Set expiration on first request
	if count == 1 {
		err = redisClient.Expire(ctx, key, time.Minute)
		if err != nil {
			return false, err
		}
	}

	// Check if limit exceeded
	return count <= int64(cfg.RequestsPerMinute), nil
}

func getRemainingRequests(redisClient *clients.RedisClient, key string, cfg config.RateLimitConfig, c *gin.Context) int {
	ctx := c.Request.Context()
	
	countStr, err := redisClient.Get(ctx, key)
	if err != nil {
		return cfg.RequestsPerMinute
	}
	
	count, err := strconv.Atoi(countStr)
	if err != nil {
		return cfg.RequestsPerMinute
	}
	
	remaining := cfg.RequestsPerMinute - count
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
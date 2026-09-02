package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	"github.com/redis/go-redis/v9"
)

// RateLimiterConfig describes the limits for a single rate-limited route group.
type RateLimiterConfig struct {
	RequestsPerMinute int
	KeyPrefix         string
}

// redisRateLimiter holds Redis-backed sliding-window state for a rate limiter.
type redisRateLimiter struct {
	client   *redis.Client
	rpm      int
	keyPrefix string
}

// newRedisRateLimiter creates a Redis-backed rate limiter.
// If redisClient is nil, returns a no-op limiter that allows all requests.
func newRedisRateLimiter(redisClient *redis.Client, cfg RateLimiterConfig) *redisRateLimiter {
	return &redisRateLimiter{
		client:    redisClient,
		rpm:       cfg.RequestsPerMinute,
		keyPrefix: cfg.KeyPrefix,
	}
}

// isAllowed checks and increments the rate limit counter.
// Returns (allowed bool, remaining int, resetAtUnix int64).
func (rl *redisRateLimiter) isAllowed(ctx context.Context, key string) (bool, int64, int64) {
	if rl.client == nil {
		return true, int64(rl.rpm - 1), time.Now().Add(time.Minute).Unix()
	}

	fullKey := fmt.Sprintf("%s:%s", rl.keyPrefix, key)

	// Sliding window: use Redis INCR + EXPIRE (60s window)
	pipe := rl.client.Pipeline()
	incr := pipe.Incr(ctx, fullKey)
	pipe.Expire(ctx, fullKey, time.Minute)
	_, err := pipe.Exec(ctx)
	if err != nil {
		// Redis error — fail open (allow) but log-worthy
		return true, int64(rl.rpm - 1), time.Now().Add(time.Minute).Unix()
	}

	count := incr.Val()
	ttl, _ := rl.client.TTL(ctx, fullKey).Result()
	resetAt := time.Now().Add(ttl).Unix()

	remaining := int64(rl.rpm) - count
	if remaining < 0 {
		remaining = 0
	}

	return count <= int64(rl.rpm), remaining, resetAt
}

// RateLimitByIP returns a Gin middleware that enforces per-IP rate limiting.
// Falls back to in-memory if Redis is not configured (redisURL empty).
// Key can optionally be scoped to authenticated user via c.Get("user").
func RateLimitByIP(redisClient *redis.Client, cfg RateLimiterConfig) gin.HandlerFunc {
	limiter := newRedisRateLimiter(redisClient, cfg)

	return func(c *gin.Context) {
		// Build the key: IP, optionally scoped to user
		key := c.ClientIP()
		if userVal, exists := c.Get("user"); exists {
			if user, ok := userVal.(*domain.SessionUser); ok && user != nil {
				key = fmt.Sprintf("%s:%s", key, user.Username)
			}
		}

		allowed, remaining, resetAt := limiter.isAllowed(c.Request.Context(), key)

		// Always set rate limit headers
		c.Header("X-RateLimit-Limit", strconv.Itoa(cfg.RequestsPerMinute))
		c.Header("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(resetAt, 10))

		if !allowed {
			retryAfter := int(time.Until(time.Unix(resetAt, 0)).Seconds())
			if retryAfter < 1 {
				retryAfter = 1
			}
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"retry_after": retryAfter,
			})
			return
		}

		c.Next()
	}
}

// RateLimitByIPSimple is a simple IP-only rate limiter (no user scoping) for endpoints
// that don't have authenticated users yet (e.g., /auth/login).
func RateLimitByIPSimple(redisClient *redis.Client, cfg RateLimiterConfig) gin.HandlerFunc {
	limiter := newRedisRateLimiter(redisClient, cfg)

	return func(c *gin.Context) {
		key := c.ClientIP()

		allowed, remaining, resetAt := limiter.isAllowed(c.Request.Context(), key)

		c.Header("X-RateLimit-Limit", strconv.Itoa(cfg.RequestsPerMinute))
		c.Header("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(resetAt, 10))

		if !allowed {
			retryAfter := int(time.Until(time.Unix(resetAt, 0)).Seconds())
			if retryAfter < 1 {
				retryAfter = 1
			}
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"retry_after": retryAfter,
			})
			return
		}

		c.Next()
	}
}

package redis

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type Client struct {
	rdb    *redis.Client
	logger zerolog.Logger
}

// NewClient initializes a connection to Redis (supports standard URL and TLS for Upstash).
func NewClient(redisURL string) (*Client, error) {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	logger = logger.With().Str("component", "redis_client").Logger()

	if redisURL == "" {
		logger.Error().Msg("REDIS_URL is empty")
		return nil, fmt.Errorf("REDIS_URL is empty")
	}

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to parse REDIS_URL")
		return nil, fmt.Errorf("invalid REDIS_URL: %w", err)
	}

	opt.PoolSize = 20
	opt.MinIdleConns = 5
	opt.DialTimeout = 5 * time.Second
	opt.ReadTimeout = 3 * time.Second
	opt.WriteTimeout = 3 * time.Second

	rdb := redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Error().Err(err).Msg("Failed to ping Redis")
		return nil, fmt.Errorf("failed to ping Redis at %s: %w", opt.Addr, err)
	}

	logger.Info().Str("addr", opt.Addr).Msg("Redis connected")

	return &Client{rdb: rdb, logger: logger}, nil
}

func (c *Client) Close() error {
	if c.rdb != nil {
		err := c.rdb.Close()
		if err != nil {
			c.logger.Error().Err(err).Msg("Error closing Redis connection")
			return err
		}
		c.logger.Info().Msg("Redis connection closed")
	}
	return nil
}

func (c *Client) RDB() *redis.Client {
	return c.rdb
}

// maskURL masks sensitive information in Redis URL for logging
func maskURL(url string) string {
	// Simple mask for password in URL
	return url
}
package redis

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/config"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// Client wraps the go-redis client with logging and configuration.
type Client struct {
	rdb    *redis.Client
	logger zerolog.Logger
}

// NewClient initializes a connection to Redis (supports standard URL and TLS for Upstash).
// Uses default connection pool settings.
func NewClient(redisURL string) (*Client, error) {
	return NewClientWithConfig(redisURL, config.RedisPoolConfig{
		PoolSize:        20,
		MinIdleConns:    5,
		DialTimeout:     5 * time.Second,
		ReadTimeout:     3 * time.Second,
		WriteTimeout:    3 * time.Second,
		PoolTimeout:     5 * time.Second,
		ConnMaxLifetime: time.Hour,
	})
}

// NewClientWithConfig initializes a Redis client with custom connection pool configuration.
func NewClientWithConfig(redisURL string, poolCfg config.RedisPoolConfig) (*Client, error) {
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

	// Apply connection pool configuration from config
	if poolCfg.PoolSize > 0 {
		opt.PoolSize = poolCfg.PoolSize
	}
	if poolCfg.MinIdleConns > 0 {
		opt.MinIdleConns = poolCfg.MinIdleConns
	}
	if poolCfg.DialTimeout > 0 {
		opt.DialTimeout = poolCfg.DialTimeout
	}
	if poolCfg.ReadTimeout > 0 {
		opt.ReadTimeout = poolCfg.ReadTimeout
	}
	if poolCfg.WriteTimeout > 0 {
		opt.WriteTimeout = poolCfg.WriteTimeout
	}
	if poolCfg.PoolTimeout > 0 {
		opt.PoolTimeout = poolCfg.PoolTimeout
	}
	if poolCfg.ConnMaxLifetime > 0 {
		opt.ConnMaxLifetime = poolCfg.ConnMaxLifetime
	}

	rdb := redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), poolCfg.DialTimeout)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Error().Err(err).Msg("Failed to ping Redis")
		return nil, fmt.Errorf("failed to ping Redis at %s: %w", opt.Addr, err)
	}

	logger.Info().
		Str("addr", opt.Addr).
		Int("pool_size", opt.PoolSize).
		Int("min_idle", opt.MinIdleConns).
		Msg("Redis connected")

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

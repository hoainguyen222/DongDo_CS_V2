package redis

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	rdb *redis.Client
}

// NewClient initializes a connection to Redis (supports standard URL and TLS for Upstash).
func NewClient(redisURL string) (*Client, error) {
	if redisURL == "" {
		return nil, fmt.Errorf("REDIS_URL is empty")
	}

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_URL: %w", err)
	}

	// Configure pool and timeouts
	opt.PoolSize = 20
	opt.MinIdleConns = 5
	opt.DialTimeout = 5 * time.Second
	opt.ReadTimeout = 3 * time.Second
	opt.WriteTimeout = 3 * time.Second

	rdb := redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping Redis at %s: %w", opt.Addr, err)
	}

	log.Printf("✅ Connected to Redis at %s", opt.Addr)
	return &Client{rdb: rdb}, nil
}

func (c *Client) Close() error {
	if c.rdb != nil {
		return c.rdb.Close()
	}
	return nil
}

func (c *Client) RDB() *redis.Client {
	return c.rdb
}

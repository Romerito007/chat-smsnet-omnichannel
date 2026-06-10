// Package redis owns the shared go-redis client used for cache, presence,
// distributed locks, rate limiting and pub/sub fan-out.
package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/romerito007/chat-smsnet-omnichannel/app/config"
)

// Client aliases the go-redis client so callers depend on this package.
type Client = *goredis.Client

// Connect builds a Redis client and verifies it with a ping.
func Connect(ctx context.Context, cfg config.RedisConfig) (Client, error) {
	c := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := c.Ping(pingCtx).Err(); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return c, nil
}

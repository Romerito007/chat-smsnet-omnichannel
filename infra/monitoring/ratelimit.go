package monitoring

import (
	"context"
	"time"

	mcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/monitoring/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/redis"
)

// RateLimiter is a Redis fixed-window per-tenant limiter for monitoring queries,
// protecting the upstream system.
type RateLimiter struct {
	rdb    redis.Client
	max    int
	window time.Duration
}

// NewRateLimiter builds the limiter (max requests per minute).
func NewRateLimiter(rdb redis.Client, maxPerMinute int) *RateLimiter {
	if maxPerMinute <= 0 {
		maxPerMinute = 60
	}
	return &RateLimiter{rdb: rdb, max: maxPerMinute, window: time.Minute}
}

// Allow reports whether another query is permitted. It fails open on Redis errors.
func (l *RateLimiter) Allow(ctx context.Context, tenantID string) (bool, error) {
	key := "monitoring:rl:" + tenantID
	count, err := l.rdb.Incr(ctx, key).Result()
	if err != nil {
		return true, nil
	}
	if count == 1 {
		_ = l.rdb.Expire(ctx, key, l.window).Err()
	}
	return count <= int64(l.max), nil
}

var _ mcontracts.RateLimiter = (*RateLimiter)(nil)

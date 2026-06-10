package webhooks

import (
	"context"
	"time"

	whcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/redis"
)

// RateLimiter is a Redis fixed-window per-subscription limiter for outbound
// webhook deliveries, protecting the receiver's endpoint.
type RateLimiter struct {
	rdb    redis.Client
	window time.Duration
}

// NewRateLimiter builds the limiter.
func NewRateLimiter(rdb redis.Client) *RateLimiter {
	return &RateLimiter{rdb: rdb, window: time.Minute}
}

// Allow reports whether another delivery to the webhook is permitted this
// minute. limitPerMin of 0 means unlimited. It fails open on Redis errors so a
// limiter outage never blocks deliveries.
func (l *RateLimiter) Allow(ctx context.Context, webhookID string, limitPerMin int) (bool, error) {
	if limitPerMin <= 0 {
		return true, nil
	}
	key := "webhook:rl:" + webhookID
	count, err := l.rdb.Incr(ctx, key).Result()
	if err != nil {
		return true, nil
	}
	if count == 1 {
		_ = l.rdb.Expire(ctx, key, l.window).Err()
	}
	return count <= int64(limitPerMin), nil
}

var _ whcontracts.RateLimiter = (*RateLimiter)(nil)

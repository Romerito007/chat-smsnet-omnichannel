package automationrules

import (
	"context"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/infra/redis"
)

// dedupWindow bounds how long a (rule,conversation,event) firing suppresses a
// repeat. It is the anti-loop guard: since the only action is send_webhook (which
// leaves the system and mutates no internal state), there is no internal feedback
// loop — this window only absorbs bursts and external-callback re-entrancy. If an
// internal-mutating action is ever added, revisit this guard.
const dedupWindow = 10 * time.Second

// Deduper is a Redis SETNX anti-loop guard for rule firings.
type Deduper struct {
	rdb redis.Client
}

// NewDeduper builds the deduper.
func NewDeduper(rdb redis.Client) *Deduper { return &Deduper{rdb: rdb} }

// Allow returns true the first time key is seen within the window, false on a
// repeat. It fails open (returns true) on Redis errors so an outage never blocks
// legitimate firings.
func (d *Deduper) Allow(ctx context.Context, key string) (bool, error) {
	set, err := d.rdb.SetNX(ctx, key, "1", dedupWindow).Result()
	if err != nil {
		return true, nil
	}
	return set, nil
}

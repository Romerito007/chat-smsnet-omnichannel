package automationrules

import (
	"context"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/infra/redis"
)

// dedupWindow bounds how long a (rule, event_id) firing stays claimed. The key is
// claimed BEFORE the rule's actions run, so an Asynq retry of the same task finds
// it claimed and skips — making side-effectful actions (e.g. send_message) safe
// against retries. The window only needs to outlive the task's retry horizon.
const dedupWindow = 1 * time.Hour

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

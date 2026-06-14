package automationrules

import (
	"context"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/redis"
)

// budgetMax / budgetWindow are the LAYER-2 safety fuse: at most budgetMax
// automation messages per conversation per window. This is a circuit breaker for a
// buggy integrator that echoes forever — NOT flow control. A legitimate sales
// funnel never approaches it; tripping it suppresses message/attachment actions and
// logs skipped_budget (the rule is NOT disabled).
const (
	budgetMax    = 100
	budgetWindow = 10 * time.Minute
)

// Budget is a Redis INCR window counter implementing service.BudgetLimiter.
type Budget struct {
	rdb redis.Client
}

// NewBudget builds the limiter.
func NewBudget(rdb redis.Client) *Budget { return &Budget{rdb: rdb} }

// AllowAutomationMessage increments the conversation's automation-message counter
// within the rolling window and reports whether it is still under the cap. It
// fails open (returns true) on any error so an outage never blocks a legitimate
// send.
func (b *Budget) AllowAutomationMessage(ctx context.Context, conversationID string) (bool, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return true, nil
	}
	key := "autom:budget:" + tenantID + ":" + conversationID
	n, err := b.rdb.Incr(ctx, key).Result()
	if err != nil {
		return true, nil
	}
	if n == 1 {
		_ = b.rdb.Expire(ctx, key, budgetWindow).Err()
	}
	return n <= budgetMax, nil
}

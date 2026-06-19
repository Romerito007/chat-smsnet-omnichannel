package presence

import (
	"context"
	"fmt"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/redis"
)

// ExpiryHandler marks an agent offline when its presence record vanished. The
// presence service implements it (Vanished): the watcher resolves the tenant from
// the expired key and calls it with a tenant-scoped context.
type ExpiryHandler interface {
	Vanished(ctx context.Context, userID string) error
}

// ExpiryWatcher subscribes to Redis keyspace "expired" events and, for every
// presence:conns:<tenant>:<user> connection-set key that lapses (the last WS
// heartbeat stopped renewing its TTL), asks the handler to recompute the agent's
// effective status (Vanished). This is the abrupt-disconnect FALLBACK — where the
// graceful fast-path never ran — turning a dead socket into a live offline on the
// dashboards (subject to the agent's auto-offline rule), not a stale "online".
type ExpiryWatcher struct {
	rdb     redis.Client
	db      int
	handler ExpiryHandler
	logger  shared.Logger
}

// NewExpiryWatcher builds the watcher. db is the Redis logical database index
// (keyspace events are scoped per-db: __keyevent@<db>__:expired).
func NewExpiryWatcher(rdb redis.Client, db int, handler ExpiryHandler, logger shared.Logger) *ExpiryWatcher {
	return &ExpiryWatcher{rdb: rdb, db: db, handler: handler, logger: logger}
}

// Run best-effort enables expired-key notifications, then subscribes to the
// expired-key channel and dispatches each presence key to the handler. It blocks
// until ctx is cancelled.
func (w *ExpiryWatcher) Run(ctx context.Context) error {
	// Self-configure so a fresh Redis emits expired events. Managed Redis may forbid
	// CONFIG SET — log and continue (an operator can set notify-keyspace-events Ex).
	if err := w.rdb.ConfigSet(ctx, "notify-keyspace-events", "Ex").Err(); err != nil {
		w.logger.Warn("presence expiry: could not enable keyspace notifications; presence will still expire but won't fan out live until notify-keyspace-events includes Ex",
			"error", err.Error())
	}

	channel := fmt.Sprintf("__keyevent@%d__:expired", w.db)
	sub := w.rdb.Subscribe(ctx, channel)
	defer func() { _ = sub.Close() }()

	w.logger.Info("presence expiry watcher started", "channel", channel)
	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			w.dispatch(ctx, msg.Payload)
		}
	}
}

// dispatch parses an expired key and, when it is a presence record, marks the
// agent offline.
func (w *ExpiryWatcher) dispatch(ctx context.Context, key string) {
	tenant, user, ok := parsePresenceKey(key)
	if !ok {
		return
	}
	if err := w.handler.Vanished(shared.WithTenant(ctx, tenant), user); err != nil {
		w.logger.Warn("presence expiry: vanish failed", "tenant_id", tenant, "user_id", user, "error", err.Error())
	}
}

// parsePresenceKey extracts tenant + user from an expired connection-set key
// "presence:conns:<tenant>:<user>" (the only presence key that carries a TTL).
// Tenant and user ids carry no colons, so a plain split is unambiguous.
func parsePresenceKey(key string) (tenant, user string, ok bool) {
	const prefix = "presence:conns:"
	if !strings.HasPrefix(key, prefix) {
		return "", "", false
	}
	parts := strings.Split(key, ":")
	if len(parts) != 4 || parts[2] == "" || parts[3] == "" {
		return "", "", false
	}
	return parts[2], parts[3], true
}

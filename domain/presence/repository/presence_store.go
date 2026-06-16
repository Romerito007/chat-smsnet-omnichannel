// Package repository declares the presence persistence and load-counting ports.
// PresenceStore is backed by Redis (operational state); LoadCounter reads open
// conversations to derive an agent's current load.
package repository

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/presence/entity"
)

// PresenceStore persists agent presence in Redis, scoped by the tenant on the
// context.
type PresenceStore interface {
	// Save upserts the presence record and arms its liveness TTL.
	Save(ctx context.Context, p *entity.AgentPresence) error
	// Touch renews the liveness TTL of an existing record without changing its
	// stored status. A missing record is a no-op (it is never resurrected), so a
	// connecting socket is not promoted to online and a vanished agent stays gone.
	Touch(ctx context.Context, userID string) error
	// Remove deletes the presence record and drops the agent from the tenant
	// roster (on graceful disconnect or TTL expiry). Idempotent.
	Remove(ctx context.Context, userID string) error
	// Get returns the stored presence, or a not_found AppError when absent.
	Get(ctx context.Context, userID string) (*entity.AgentPresence, error)
	// List returns presence for every agent known in the tenant.
	List(ctx context.Context) ([]*entity.AgentPresence, error)
}

// LoadCounter derives an agent's current load from open, assigned conversations.
type LoadCounter interface {
	// CountOpenAssigned counts the conversations currently assigned to userID
	// that are still open within the tenant on the context.
	CountOpenAssigned(ctx context.Context, userID string) (int, error)
	// OpenAssignedLoads returns, in ONE aggregation, the open-conversation load of
	// every assigned agent in the tenant (keyed by user id), so a presence/routing
	// listing computes loads without a count-per-agent N+1. Agents with no open
	// assigned conversation are absent from the map (load 0).
	OpenAssignedLoads(ctx context.Context) (map[string]int, error)
}

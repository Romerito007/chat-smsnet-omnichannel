package models

import "time"

// DealTimelineEvent is the BSON document for one entry on a deal's timeline. Data is
// the kind-specific, free-form payload (ids only; names are resolved at read time).
type DealTimelineEvent struct {
	ID        string         `bson:"_id"`
	TenantID  string         `bson:"tenant_id"`
	DealID    string         `bson:"deal_id"`
	Kind      string         `bson:"kind"`
	ActorID   string         `bson:"actor_id"`
	Data      map[string]any `bson:"data,omitempty"`
	CreatedAt time.Time      `bson:"created_at"`
}

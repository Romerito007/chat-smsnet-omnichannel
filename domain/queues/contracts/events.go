package contracts

import "context"

// RealtimeQueueStats is the realtime event published when a queue's composition
// changes (a conversation enters, leaves or is assigned out of the queue).
const RealtimeQueueStats = "queue.stats"

// QueueStatsPayload is the minimal queue-composition snapshot delivered to agents
// watching the sector inbox.
type QueueStatsPayload struct {
	TenantID      string `json:"tenant_id"`
	SectorID      string `json:"sector_id"`
	QueueID       string `json:"queue_id"`
	WaitingCount  int    `json:"waiting_count"`
	AssignedCount int    `json:"assigned_count"`
}

// CompositionCounter counts the conversations that make up a queue's composition.
// Implemented by the Mongo conversations collection (tenant-scoped from context).
// WaitingCount counts conversations still queued in queueID; AssignedCount counts
// conversations currently assigned within the queue's sector (queue_id is cleared
// on assignment, so assigned work is tracked by sector).
type CompositionCounter interface {
	QueueComposition(ctx context.Context, sectorID, queueID string) (waiting, assigned int, err error)
}

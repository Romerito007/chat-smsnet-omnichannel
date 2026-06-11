package shared

import "context"

// QueueStatsNotifier is signalled by producers (routing, conversations) whenever a
// queue's composition changes — a conversation enters the queue, leaves it, or is
// assigned out of it. The implementation (the queues domain) recomputes the
// waiting/assigned counts and publishes the queue.stats realtime event.
//
// Fire-and-forget: a stats failure must never break the operation that changed the
// queue. The default no-op drops the signal.
type QueueStatsNotifier interface {
	QueueChanged(ctx context.Context, sectorID, queueID string)
}

// NoopQueueStatsNotifier discards queue-change signals.
type NoopQueueStatsNotifier struct{}

// QueueChanged implements QueueStatsNotifier.
func (NoopQueueStatsNotifier) QueueChanged(context.Context, string, string) {}

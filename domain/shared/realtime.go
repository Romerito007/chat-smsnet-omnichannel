package shared

import "context"

// EventPublisher publishes realtime events to a topic for WebSocket fan-out. The
// domain depends on this port; the infra realtime manager implements it. Keeping
// it here lets any domain emit events without importing the transport.
type EventPublisher interface {
	// Publish emits a named event with a JSON-serializable payload to a topic.
	Publish(ctx context.Context, topic string, event string, data any) error
}

// PublishEvent is one event in a batch publish (topic + name + payload).
type PublishEvent struct {
	Topic string
	Event string
	Data  any
}

// BatchEventPublisher is an optional capability: emitting several events in ONE
// transport round trip (a Redis pipeline), preserving order. The realtime publisher
// implements it so the message-create path's events (message.created +
// conversation.updated…) cost a single RTT instead of N serial ones.
type BatchEventPublisher interface {
	PublishBatch(ctx context.Context, events []PublishEvent) error
}

// PublishAll emits events best-effort, in ONE round trip when pub supports batching,
// else sequentially. Order is preserved either way and errors are ignored (realtime
// is best-effort — a publish failure must never fail the operation that produced it).
func PublishAll(ctx context.Context, pub EventPublisher, events ...PublishEvent) {
	if pub == nil || len(events) == 0 {
		return
	}
	if bp, ok := pub.(BatchEventPublisher); ok {
		_ = bp.PublishBatch(ctx, events)
		return
	}
	for _, e := range events {
		_ = pub.Publish(ctx, e.Topic, e.Event, e.Data)
	}
}

// NoopPublisher discards events. Useful as a default and in tests.
type NoopPublisher struct{}

// Publish implements EventPublisher.
func (NoopPublisher) Publish(context.Context, string, string, any) error { return nil }

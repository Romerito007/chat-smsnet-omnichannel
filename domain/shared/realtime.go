package shared

import "context"

// EventPublisher publishes realtime events to a topic for WebSocket fan-out. The
// domain depends on this port; the infra realtime manager implements it. Keeping
// it here lets any domain emit events without importing the transport.
type EventPublisher interface {
	// Publish emits a named event with a JSON-serializable payload to a topic.
	Publish(ctx context.Context, topic string, event string, data any) error
}

// NoopPublisher discards events. Useful as a default and in tests.
type NoopPublisher struct{}

// Publish implements EventPublisher.
func (NoopPublisher) Publish(context.Context, string, string, any) error { return nil }

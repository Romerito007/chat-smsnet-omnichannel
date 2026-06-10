package realtime

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/infra/redis"
)

// Manager is the single entry point the rest of the app uses for realtime: it
// owns the Hub and the Redis PubSub transport and exposes a Publish method that
// services call to emit live events.
type Manager struct {
	Hub    *Hub
	pubsub *PubSub
}

// NewManager wires a Hub and PubSub over the given Redis client.
func NewManager(rdb redis.Client) *Manager {
	hub := NewHub()
	return &Manager{Hub: hub, pubsub: NewPubSub(rdb, hub)}
}

// Publish emits a topic message to every connected client across all nodes.
func (m *Manager) Publish(ctx context.Context, topic Topic, payload []byte) error {
	return m.pubsub.Publish(ctx, Message{Topic: topic, Payload: payload})
}

// Run starts the cross-node subscription loop; call it from the ws role.
func (m *Manager) Run(ctx context.Context) error {
	return m.pubsub.Run(ctx)
}

package realtime

import (
	"context"
	"encoding/json"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// EventPublisher adapts the realtime Manager to the domain's shared.EventPublisher
// port: it wraps the event in the standard envelope ({event, ts, data}) and
// fans it out across nodes via Redis Pub/Sub.
type EventPublisher struct {
	mgr *Manager
}

// NewEventPublisher builds the adapter.
func NewEventPublisher(mgr *Manager) *EventPublisher {
	return &EventPublisher{mgr: mgr}
}

// envelope is the on-the-wire shape delivered to WebSocket clients.
type envelope struct {
	Event string `json:"event"`
	Ts    int64  `json:"ts"`
	Data  any    `json:"data"`
}

// Publish serializes the event and fans it out to subscribers of the topic.
func (p *EventPublisher) Publish(ctx context.Context, topic string, event string, data any) error {
	payload, err := json.Marshal(envelope{Event: event, Ts: time.Now().UnixMilli(), Data: data})
	if err != nil {
		return err
	}
	return p.mgr.Publish(ctx, topic, payload)
}

var _ shared.EventPublisher = (*EventPublisher)(nil)

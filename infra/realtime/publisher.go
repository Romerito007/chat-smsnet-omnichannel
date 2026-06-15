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

// PublishBatch serializes several events and fans them out in ONE Redis round trip
// (a pipeline), preserving order. Used for the message-create burst (message.created
// + conversation.updated…) so the request pays a single RTT instead of N serial ones.
func (p *EventPublisher) PublishBatch(ctx context.Context, events []shared.PublishEvent) error {
	if len(events) == 0 {
		return nil
	}
	now := time.Now().UnixMilli()
	msgs := make([]Message, 0, len(events))
	for _, e := range events {
		payload, err := json.Marshal(envelope{Event: e.Event, Ts: now, Data: e.Data})
		if err != nil {
			return err
		}
		msgs = append(msgs, Message{Topic: e.Topic, Payload: payload})
	}
	return p.mgr.PublishBatch(ctx, msgs)
}

var (
	_ shared.EventPublisher      = (*EventPublisher)(nil)
	_ shared.BatchEventPublisher = (*EventPublisher)(nil)
)

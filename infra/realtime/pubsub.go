package realtime

import (
	"context"
	"encoding/json"

	"github.com/romerito007/chat-smsnet-omnichannel/infra/redis"
)

// channelName is the single Redis pub/sub channel all realtime nodes listen on.
// Topic-level routing happens in-process via the Hub; this keeps the Redis
// fan-out simple while still scaling horizontally.
const channelName = "realtime:fanout"

// PubSub bridges the local Hub with Redis so a message published on one node is
// delivered by every node.
type PubSub struct {
	rdb redis.Client
	hub *Hub
}

// NewPubSub builds the transport.
func NewPubSub(rdb redis.Client, hub *Hub) *PubSub {
	return &PubSub{rdb: rdb, hub: hub}
}

// Publish broadcasts a message to all nodes (including this one, via the
// subscription loop).
func (p *PubSub) Publish(ctx context.Context, msg Message) error {
	raw, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return p.rdb.Publish(ctx, channelName, raw).Err()
}

// PublishBatch broadcasts several messages in a single Redis pipeline (one round
// trip), preserving order. A marshal failure aborts before any network write.
func (p *PubSub) PublishBatch(ctx context.Context, msgs []Message) error {
	if len(msgs) == 0 {
		return nil
	}
	raws := make([][]byte, len(msgs))
	for i, m := range msgs {
		raw, err := json.Marshal(m)
		if err != nil {
			return err
		}
		raws[i] = raw
	}
	pipe := p.rdb.Pipeline()
	for _, raw := range raws {
		pipe.Publish(ctx, channelName, raw)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// Run subscribes to the fan-out channel and delivers received messages to local
// clients until the context is cancelled.
func (p *PubSub) Run(ctx context.Context) error {
	sub := p.rdb.Subscribe(ctx, channelName)
	defer func() { _ = sub.Close() }()

	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case raw, ok := <-ch:
			if !ok {
				return nil
			}
			var msg Message
			if err := json.Unmarshal([]byte(raw.Payload), &msg); err != nil {
				continue
			}
			p.hub.Deliver(msg)
		}
	}
}

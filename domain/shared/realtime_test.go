package shared

import (
	"context"
	"testing"
)

type seqPublisher struct{ events []string } // only implements Publish (fallback path)

func (p *seqPublisher) Publish(_ context.Context, topic, event string, _ any) error {
	p.events = append(p.events, event+"@"+topic)
	return nil
}

type batchPublisher struct {
	seqPublisher
	batches int
}

func (p *batchPublisher) PublishBatch(_ context.Context, events []PublishEvent) error {
	p.batches++
	for _, e := range events {
		p.events = append(p.events, e.Event+"@"+e.Topic)
	}
	return nil
}

// TestPublishAll_UsesBatchWhenSupported: a batch-capable publisher gets ONE
// PublishBatch call with all events in order (the 1-RTT pipeline path).
func TestPublishAll_UsesBatchWhenSupported(t *testing.T) {
	p := &batchPublisher{}
	PublishAll(context.Background(), p,
		PublishEvent{Topic: "conv:1", Event: "message.created"},
		PublishEvent{Topic: "conv:1", Event: "conversation.updated"},
		PublishEvent{Topic: "inbox:s1", Event: "conversation.updated"},
	)
	if p.batches != 1 {
		t.Fatalf("expected exactly one batch (1 RTT), got %d", p.batches)
	}
	want := []string{"message.created@conv:1", "conversation.updated@conv:1", "conversation.updated@inbox:s1"}
	if len(p.events) != len(want) {
		t.Fatalf("events = %v", p.events)
	}
	for i := range want {
		if p.events[i] != want[i] {
			t.Errorf("order[%d] = %q, want %q (message.created must precede conversation.updated)", i, p.events[i], want[i])
		}
	}
}

// TestPublishAll_FallsBackToSequential: a publisher without batching still gets all
// events, in order, via individual Publish calls (back-compat, e.g. test fakes).
func TestPublishAll_FallsBackToSequential(t *testing.T) {
	p := &seqPublisher{}
	PublishAll(context.Background(), p,
		PublishEvent{Topic: "conv:1", Event: "message.created"},
		PublishEvent{Topic: "conv:1", Event: "conversation.updated"},
	)
	if len(p.events) != 2 || p.events[0] != "message.created@conv:1" || p.events[1] != "conversation.updated@conv:1" {
		t.Fatalf("fallback must publish all events in order, got %v", p.events)
	}
}

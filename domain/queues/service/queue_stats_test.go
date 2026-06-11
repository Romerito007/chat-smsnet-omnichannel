package service

import (
	"context"
	"errors"
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/queues/contracts"
)

type capturedStats struct {
	topic   string
	event   string
	payload contracts.QueueStatsPayload
}

type fakeStatsPublisher struct{ got []capturedStats }

func (p *fakeStatsPublisher) Publish(_ context.Context, topic, event string, payload any) error {
	ps, _ := payload.(contracts.QueueStatsPayload)
	p.got = append(p.got, capturedStats{topic: topic, event: event, payload: ps})
	return nil
}

type fakeCounter struct {
	waiting, assigned int
	err               error
}

func (c fakeCounter) QueueComposition(context.Context, string, string) (int, int, error) {
	return c.waiting, c.assigned, c.err
}

func TestQueueChanged_PublishesQueueStats(t *testing.T) {
	svc := newQueueService(map[string]string{"s1": "t1"})
	pub := &fakeStatsPublisher{}
	svc.SetStats(pub, fakeCounter{waiting: 3, assigned: 2})

	svc.QueueChanged(tenantCtx("t1"), "s1", "q1")

	if len(pub.got) != 1 {
		t.Fatalf("expected one queue.stats publish, got %d", len(pub.got))
	}
	e := pub.got[0]
	if e.event != contracts.RealtimeQueueStats {
		t.Errorf("event = %q, want queue.stats", e.event)
	}
	p := e.payload
	if p.TenantID != "t1" || p.SectorID != "s1" || p.QueueID != "q1" || p.WaitingCount != 3 || p.AssignedCount != 2 {
		t.Errorf("unexpected payload: %+v", p)
	}
}

func TestQueueChanged_NoCounterOrNoQueueIsNoop(t *testing.T) {
	svc := newQueueService(map[string]string{"s1": "t1"})
	pub := &fakeStatsPublisher{}

	// No counter wired → no-op.
	svc.QueueChanged(tenantCtx("t1"), "s1", "q1")

	// Counter wired but empty queue id → no-op.
	svc.SetStats(pub, fakeCounter{waiting: 1})
	svc.QueueChanged(tenantCtx("t1"), "s1", "")

	if len(pub.got) != 0 {
		t.Errorf("expected no publish, got %+v", pub.got)
	}
}

func TestQueueChanged_CounterErrorSwallowed(t *testing.T) {
	svc := newQueueService(map[string]string{"s1": "t1"})
	pub := &fakeStatsPublisher{}
	svc.SetStats(pub, fakeCounter{err: errors.New("boom")})

	// Must not panic and must not publish on counter error.
	svc.QueueChanged(tenantCtx("t1"), "s1", "q1")
	if len(pub.got) != 0 {
		t.Errorf("expected no publish on counter error, got %+v", pub.got)
	}
}

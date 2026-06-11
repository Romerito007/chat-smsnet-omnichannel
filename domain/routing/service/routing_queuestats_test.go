package service

import (
	"context"
	"testing"

	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	iamentity "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/entity"
	presenceentity "github.com/romerito007/chat-smsnet-omnichannel/domain/presence/entity"
	queueentity "github.com/romerito007/chat-smsnet-omnichannel/domain/queues/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/routing/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

type fakeRoutingQueueStats struct {
	calls []struct{ sector, queue string }
}

func (f *fakeRoutingQueueStats) QueueChanged(_ context.Context, sectorID, queueID string) {
	f.calls = append(f.calls, struct{ sector, queue string }{sectorID, queueID})
}

func (f *fakeRoutingQueueStats) saw(sector, queue string) bool {
	for _, c := range f.calls {
		if c.sector == sector && c.queue == queue {
			return true
		}
	}
	return false
}

func TestEnqueue_NotifiesQueueStats(t *testing.T) {
	queues := map[string]*queueentity.Queue{
		"q1": {ID: "q1", TenantID: "t1", SectorID: "s9", Name: "Q", Strategy: queueentity.StrategyManual},
	}
	conv := convNew("conv1", "s1")
	fx := newFixture(shared.NoopLocker{}, &fakeUsers{byID: map[string]*iamentity.User{}},
		&fakePresence{byUser: map[string]*presenceentity.AgentPresence{}}, nil,
		map[string]*conventity.Conversation{"conv1": conv}, queues, nil)
	qs := &fakeRoutingQueueStats{}
	fx.svc.SetQueueStatsNotifier(qs)

	if _, err := fx.svc.Enqueue(adminCtx(), "conv1", contracts.EnqueueCommand{QueueID: "q1"}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if !qs.saw("s9", "q1") {
		t.Errorf("expected QueueChanged(s9,q1) on enqueue, got %+v", qs.calls)
	}
}

func TestAssign_NotifiesQueueStatsForLeftQueue(t *testing.T) {
	users := &fakeUsers{byID: map[string]*iamentity.User{"a": agent("a", "s1", 5)}}
	presence := &fakePresence{byUser: map[string]*presenceentity.AgentPresence{
		"a": presenceOf("a", presenceentity.StatusAvailable, 1, 5),
	}}
	conv := convNew("conv1", "s1")
	conv.QueueID = "q1"
	conv.Status = conventity.StatusQueued
	fx := newFixture(shared.NoopLocker{}, users, presence, map[string]int{"a": 1},
		map[string]*conventity.Conversation{"conv1": conv}, nil, nil)
	qs := &fakeRoutingQueueStats{}
	fx.svc.SetQueueStatsNotifier(qs)

	if _, err := fx.svc.Assign(adminCtx(), "conv1", "a"); err != nil {
		t.Fatalf("assign: %v", err)
	}
	if !qs.saw("s1", "q1") {
		t.Errorf("expected QueueChanged(s1,q1) when assignment empties the queue, got %+v", qs.calls)
	}
}

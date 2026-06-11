package service

import (
	"context"
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
)

// published reports whether the fake publisher saw the given event name.
func published(pub *fakePublisher, event string) bool {
	for _, e := range pub.events {
		if e.event == event {
			return true
		}
	}
	return false
}

// fakeQueueStats records QueueChanged signals.
type fakeQueueStats struct {
	calls []struct{ sector, queue string }
}

func (f *fakeQueueStats) QueueChanged(_ context.Context, sectorID, queueID string) {
	f.calls = append(f.calls, struct{ sector, queue string }{sectorID, queueID})
}

func TestCreate_PublishesConversationCreated(t *testing.T) {
	svc, _, _, _, pub := newService(map[string]string{"s1": "t1"})
	if _, err := svc.Create(adminCtx(), contracts.CreateConversation{ContactID: "c1", Channel: "wa", SectorID: "s1"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if !published(pub, contracts.RealtimeConversationCreated) {
		t.Errorf("expected conversation.created, got %+v", pub.events)
	}
	if !published(pub, contracts.RealtimeConversationUpdated) {
		t.Errorf("conversation.updated must still be published (backward compat)")
	}
}

func TestClose_PublishesConversationClosed(t *testing.T) {
	svc, _, _, _, pub := newService(map[string]string{"s1": "t1"})
	ctx := adminCtx()
	conv, _ := svc.Create(ctx, contracts.CreateConversation{ContactID: "c1", Channel: "wa", SectorID: "s1"})
	pub.events = nil
	if _, err := svc.Close(ctx, conv.ID, contracts.CloseConversation{}); err != nil {
		t.Fatalf("close: %v", err)
	}
	if !published(pub, contracts.RealtimeConversationClosed) {
		t.Errorf("expected conversation.closed, got %+v", pub.events)
	}
}

func TestReopen_PublishesConversationReopened(t *testing.T) {
	svc, _, _, _, pub := newService(map[string]string{"s1": "t1"})
	ctx := adminCtx()
	conv, _ := svc.Create(ctx, contracts.CreateConversation{ContactID: "c1", Channel: "wa", SectorID: "s1"})
	if _, err := svc.Close(ctx, conv.ID, contracts.CloseConversation{}); err != nil {
		t.Fatalf("close: %v", err)
	}
	pub.events = nil
	if _, err := svc.Reopen(ctx, conv.ID); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if !published(pub, contracts.RealtimeConversationReopened) {
		t.Errorf("expected conversation.reopened, got %+v", pub.events)
	}
}

func TestUpdate_StatusResolved_PublishesResolved(t *testing.T) {
	svc, _, _, _, pub := newService(map[string]string{"s1": "t1"})
	ctx := adminCtx()
	conv, _ := svc.Create(ctx, contracts.CreateConversation{ContactID: "c1", Channel: "wa", SectorID: "s1"})
	pub.events = nil
	resolved := entity.StatusResolved
	if _, err := svc.Update(ctx, conv.ID, contracts.UpdateConversation{Status: &resolved}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if !published(pub, contracts.RealtimeConversationResolved) {
		t.Errorf("expected conversation.resolved, got %+v", pub.events)
	}
}

func TestCreate_IntoQueue_NotifiesQueueStats(t *testing.T) {
	svc, _, _, _, _ := newService(map[string]string{"s1": "t1"})
	qs := &fakeQueueStats{}
	svc.SetQueueStatsNotifier(qs)
	if _, err := svc.Create(adminCtx(), contracts.CreateConversation{
		ContactID: "c1", Channel: "wa", SectorID: "s1", QueueID: "q1",
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(qs.calls) != 1 || qs.calls[0].queue != "q1" || qs.calls[0].sector != "s1" {
		t.Errorf("expected QueueChanged(s1,q1), got %+v", qs.calls)
	}
}

func TestClose_QueuedNotifiesQueueStats(t *testing.T) {
	svc, _, _, _, _ := newService(map[string]string{"s1": "t1"})
	qs := &fakeQueueStats{}
	svc.SetQueueStatsNotifier(qs)
	ctx := adminCtx()
	conv, _ := svc.Create(ctx, contracts.CreateConversation{ContactID: "c1", Channel: "wa", SectorID: "s1", QueueID: "q1"})
	qs.calls = nil
	if _, err := svc.Close(ctx, conv.ID, contracts.CloseConversation{}); err != nil {
		t.Fatalf("close: %v", err)
	}
	if len(qs.calls) != 1 || qs.calls[0].queue != "q1" {
		t.Errorf("expected QueueChanged on close of queued conversation, got %+v", qs.calls)
	}
}

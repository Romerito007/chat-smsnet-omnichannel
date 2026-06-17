package service

import (
	"context"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/entity"
)

func emitClock() fixedClock { return fixedClock{t: time.Unix(1700000000, 0).UTC()} }

func TestEmitTo_DeliversToSpecificWebhookBypassingEventFilter(t *testing.T) {
	// The webhook subscribes only to "something_else" AND is scoped to another
	// sector; EmitTo must still deliver the rule's event to it — the rule decides,
	// not the subscription's events[] NOR its scopes.
	subs := &fakeSubs{byID: map[string]*entity.WebhookSubscription{
		"wh1": {ID: "wh1", TenantID: "t1", Enabled: true, Events: []string{"something_else"}, Scopes: []string{"other-sector"}},
	}}
	del := newFakeDeliveries()
	enq := &fakeEnqueuer{}
	d := NewDispatcher(subs, del, enq, emitClock())

	ctx := shared.WithTenant(context.Background(), "t1")
	if err := d.EmitTo(ctx, "t1", "wh1", "conversation_created", map[string]any{"id": "cv1"}); err != nil {
		t.Fatalf("emitto: %v", err)
	}
	if len(del.created) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(del.created))
	}
	for _, dv := range del.created {
		if dv.WebhookID != "wh1" || dv.Event != "conversation_created" {
			t.Errorf("delivery wrong: %+v", dv)
		}
		if len(dv.Payload) == 0 {
			t.Errorf("envelope payload empty")
		}
	}
	if len(enq.items) != 1 || enq.items[0].task.DeliveryID == "" {
		t.Errorf("delivery not enqueued: %+v", enq.items)
	}
}

func TestEmitTo_DisabledWebhookErrors(t *testing.T) {
	subs := &fakeSubs{byID: map[string]*entity.WebhookSubscription{
		"wh1": {ID: "wh1", TenantID: "t1", Enabled: false},
	}}
	d := NewDispatcher(subs, newFakeDeliveries(), &fakeEnqueuer{}, emitClock())
	err := d.EmitTo(shared.WithTenant(context.Background(), "t1"), "t1", "wh1", "conversation_created", nil)
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("expected validation error for disabled webhook, got %v", err)
	}
}

func TestEmitToChannel_DeliversToManagedWebhook(t *testing.T) {
	// The channel's managed webhook is found by OwnedByChannelID; the event is
	// delivered to it (reusing EmitTo's pipeline) regardless of its events[] filter.
	subs := &fakeSubs{byID: map[string]*entity.WebhookSubscription{
		"wh1": {ID: "wh1", TenantID: "t1", Enabled: true, OwnedByChannelID: "ch1"},
	}}
	del := newFakeDeliveries()
	enq := &fakeEnqueuer{}
	d := NewDispatcher(subs, del, enq, emitClock())

	ctx := shared.WithTenant(context.Background(), "t1")
	if err := d.EmitToChannel(ctx, "t1", "ch1", "group_sync_requested", map[string]any{"channel_id": "ch1"}); err != nil {
		t.Fatalf("emit to channel: %v", err)
	}
	if len(del.created) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(del.created))
	}
	for _, dv := range del.created {
		if dv.WebhookID != "wh1" || dv.Event != "group_sync_requested" {
			t.Errorf("delivery wrong: %+v", dv)
		}
	}
}

func TestEmitToChannel_NoManagedWebhookConflicts(t *testing.T) {
	// A channel with no managed webhook (no outbound_url) → a clear conflict error,
	// not a silent no-op: the sync request has nowhere to go.
	subs := &fakeSubs{byID: map[string]*entity.WebhookSubscription{}}
	d := NewDispatcher(subs, newFakeDeliveries(), &fakeEnqueuer{}, emitClock())
	err := d.EmitToChannel(shared.WithTenant(context.Background(), "t1"), "t1", "ch1", "group_sync_requested", nil)
	if apperror.From(err).Code != apperror.CodeConflict {
		t.Fatalf("expected conflict for missing managed webhook, got %v", err)
	}
}

// fakeUsage is a WebhookUsageChecker stub.
type fakeUsage struct {
	inUse  bool
	usedBy string
}

func (f fakeUsage) IsWebhookInUse(context.Context, string) (bool, string, error) {
	return f.inUse, f.usedBy, nil
}

func TestSubscriptionDelete_BlockedWhenInUseByRule(t *testing.T) {
	subs := &fakeSubs{byID: map[string]*entity.WebhookSubscription{
		"wh1": {ID: "wh1", TenantID: "t1", Enabled: true},
	}}
	svc := NewSubscriptionService(subs, newFakeDeliveries(), &fakeSender{status: 200}, emitClock())
	svc.SetUsageChecker(fakeUsage{inUse: true, usedBy: "Regra de boas-vindas"})

	err := svc.Delete(shared.WithTenant(context.Background(), "t1"), "wh1")
	if apperror.From(err).Code != apperror.CodeConflict {
		t.Fatalf("expected conflict when webhook is in use by a rule, got %v", err)
	}
}

func TestSubscriptionDelete_AllowedWhenNotInUse(t *testing.T) {
	subs := &fakeSubs{byID: map[string]*entity.WebhookSubscription{
		"wh1": {ID: "wh1", TenantID: "t1", Enabled: true},
	}}
	svc := NewSubscriptionService(subs, newFakeDeliveries(), &fakeSender{status: 200}, emitClock())
	svc.SetUsageChecker(fakeUsage{inUse: false})
	if err := svc.Delete(shared.WithTenant(context.Background(), "t1"), "wh1"); err != nil {
		t.Fatalf("delete should succeed when not in use: %v", err)
	}
}

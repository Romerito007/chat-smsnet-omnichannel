package service

import (
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/entity"
)

func managedSvc() (*SubscriptionService, *fakeSubs) {
	subs := &fakeSubs{byID: map[string]*entity.WebhookSubscription{}}
	return NewSubscriptionService(subs, newFakeDeliveries(), &fakeSender{status: 200}, fixedClock{t: time.Unix(1700000000, 0).UTC()}), subs
}

func TestSyncChannelWebhook_CreatesUpdatesAndRemoves(t *testing.T) {
	svc, subs := managedSvc()
	ctx := ctxTenant()

	// First sync creates the managed subscription.
	if err := svc.SyncChannelWebhook(ctx, "ch1", "https://hook.example/in", "sek"); err != nil {
		t.Fatalf("sync create: %v", err)
	}
	got, err := subs.FindByChannelID(ctx, "ch1")
	if err != nil {
		t.Fatalf("expected a managed sub, got %v", err)
	}
	if !got.Managed() || got.URL != "https://hook.example/in" || got.Secret != "sek" {
		t.Fatalf("managed sub wrong: %+v", got)
	}
	if len(got.Events) == 0 {
		t.Fatalf("managed sub should carry the managed event set")
	}

	// Second sync updates URL + secret in place (no duplicate).
	if err := svc.SyncChannelWebhook(ctx, "ch1", "https://hook.example/v2", "sek2"); err != nil {
		t.Fatalf("sync update: %v", err)
	}
	got, _ = subs.FindByChannelID(ctx, "ch1")
	if got.URL != "https://hook.example/v2" || got.Secret != "sek2" {
		t.Fatalf("managed sub not updated: %+v", got)
	}

	// Empty URL removes it.
	if err := svc.SyncChannelWebhook(ctx, "ch1", "", "sek2"); err != nil {
		t.Fatalf("sync remove: %v", err)
	}
	if _, err := subs.FindByChannelID(ctx, "ch1"); apperror.From(err).Code != apperror.CodeNotFound {
		t.Fatalf("expected managed sub removed, got %v", err)
	}
}

func TestManagedSubscription_NotEditableThroughAPI(t *testing.T) {
	svc, subs := managedSvc()
	ctx := ctxTenant()
	if err := svc.SyncChannelWebhook(ctx, "ch1", "https://hook.example/in", "sek"); err != nil {
		t.Fatalf("sync: %v", err)
	}
	managed, _ := subs.FindByChannelID(ctx, "ch1")

	name := "hijack"
	if _, err := svc.Update(ctx, managed.ID, contracts.UpdateSubscription{Name: &name}); apperror.From(err).Code != apperror.CodeConflict {
		t.Fatalf("managed update must conflict, got %v", err)
	}
	if err := svc.Delete(ctx, managed.ID); apperror.From(err).Code != apperror.CodeConflict {
		t.Fatalf("managed delete must conflict, got %v", err)
	}
}

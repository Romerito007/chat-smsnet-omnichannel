package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/entity"
)

// ── fakes ────────────────────────────────────────────────────────────────────

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type fakeSubs struct {
	byID    map[string]*entity.WebhookSubscription
	byEvent map[string][]*entity.WebhookSubscription
}

func (r *fakeSubs) Create(_ context.Context, s *entity.WebhookSubscription) error {
	if r.byID == nil {
		r.byID = map[string]*entity.WebhookSubscription{}
	}
	cp := *s
	r.byID[s.ID] = &cp
	return nil
}
func (r *fakeSubs) Update(_ context.Context, s *entity.WebhookSubscription) error {
	if r.byID != nil {
		cp := *s
		r.byID[s.ID] = &cp
	}
	return nil
}
func (r *fakeSubs) Delete(_ context.Context, id string) error {
	delete(r.byID, id)
	return nil
}
func (r *fakeSubs) FindByID(_ context.Context, id string) (*entity.WebhookSubscription, error) {
	if s, ok := r.byID[id]; ok {
		return s, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeSubs) List(context.Context, shared.PageRequest) ([]*entity.WebhookSubscription, error) {
	return nil, nil
}
func (r *fakeSubs) ListEnabledByEvent(_ context.Context, _ string, event string) ([]*entity.WebhookSubscription, error) {
	return r.byEvent[event], nil
}
func (r *fakeSubs) FindByChannelID(_ context.Context, channelID string) (*entity.WebhookSubscription, error) {
	for _, s := range r.byID {
		if s.OwnedByChannelID == channelID {
			return s, nil
		}
	}
	return nil, apperror.NotFound("nf")
}

type fakeDeliveries struct {
	created map[string]*entity.WebhookDelivery
	updates int
}

func newFakeDeliveries() *fakeDeliveries {
	return &fakeDeliveries{created: map[string]*entity.WebhookDelivery{}}
}
func (r *fakeDeliveries) Create(_ context.Context, d *entity.WebhookDelivery) error {
	cp := *d
	r.created[d.ID] = &cp
	return nil
}
func (r *fakeDeliveries) Update(_ context.Context, d *entity.WebhookDelivery) error {
	r.updates++
	cp := *d
	r.created[d.ID] = &cp
	return nil
}
func (r *fakeDeliveries) FindByID(_ context.Context, id string) (*entity.WebhookDelivery, error) {
	if d, ok := r.created[id]; ok {
		cp := *d
		return &cp, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeDeliveries) ListByWebhook(context.Context, string, shared.PageRequest) ([]*entity.WebhookDelivery, error) {
	return nil, nil
}

type enqueued struct {
	task  contracts.DeliverTask
	delay int
	retry bool
}

type fakeEnqueuer struct{ items []enqueued }

func (e *fakeEnqueuer) EnqueueDeliver(t contracts.DeliverTask) error {
	e.items = append(e.items, enqueued{task: t})
	return nil
}
func (e *fakeEnqueuer) EnqueueRetry(t contracts.DeliverTask, delay int) error {
	e.items = append(e.items, enqueued{task: t, delay: delay, retry: true})
	return nil
}

type fakeSender struct {
	status int
	err    error
	calls  int
	gotSig bool
}

func (s *fakeSender) Send(_ context.Context, sub *entity.WebhookSubscription, d *entity.WebhookDelivery) (contracts.SendResult, error) {
	s.calls++
	s.gotSig = sub.Secret != "" && len(d.Payload) > 0
	return contracts.SendResult{StatusCode: s.status}, s.err
}

type fakeLimiter struct{ allow bool }

func (l fakeLimiter) Allow(context.Context, string, int) (bool, error) { return l.allow, nil }

func ctxTenant() context.Context { return shared.WithTenant(context.Background(), "t1") }

// ── dispatcher ───────────────────────────────────────────────────────────────

func TestDispatcher_EmitCreatesDeliveryAndEnqueues(t *testing.T) {
	sub := &entity.WebhookSubscription{ID: "wh1", TenantID: "t1", Enabled: true, Events: []string{entity.EventConversationCreated}}
	subs := &fakeSubs{byEvent: map[string][]*entity.WebhookSubscription{entity.EventConversationCreated: {sub}}}
	del := newFakeDeliveries()
	enq := &fakeEnqueuer{}
	d := NewDispatcher(subs, del, enq, fixedClock{t: time.Unix(1700000000, 0).UTC()})

	// Emit takes the INTERNAL event key (dot); the dispatcher maps it to the wire
	// name (entity.EventConversationCreated) that the subscription registered for.
	d.Emit(ctxTenant(), "t1", "conversation.created", "", map[string]any{"id": "conv1"})

	if len(del.created) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(del.created))
	}
	if len(enq.items) != 1 || enq.items[0].retry {
		t.Fatalf("expected 1 immediate enqueue, got %+v", enq.items)
	}
	for _, dlv := range del.created {
		if dlv.Status != entity.DeliveryPending || dlv.WebhookID != "wh1" || len(dlv.Payload) == 0 {
			t.Errorf("unexpected delivery: %+v", dlv)
		}
	}
}

func TestDispatcher_SectorScopeFilters(t *testing.T) {
	scoped := &entity.WebhookSubscription{ID: "scoped", TenantID: "t1", Enabled: true,
		Events: []string{entity.EventConversationCreated}, Scopes: []string{"s1"}}
	all := &entity.WebhookSubscription{ID: "all", TenantID: "t1", Enabled: true,
		Events: []string{entity.EventConversationCreated}} // no scopes = every sector
	subs := &fakeSubs{byEvent: map[string][]*entity.WebhookSubscription{
		entity.EventConversationCreated: {scoped, all},
	}}

	webhookIDs := func(sectorID string) []string {
		del := newFakeDeliveries()
		d := NewDispatcher(subs, del, &fakeEnqueuer{}, fixedClock{t: time.Unix(1700000000, 0).UTC()})
		d.Emit(ctxTenant(), "t1", "conversation.created", sectorID, map[string]any{"id": "c1"})
		ids := make([]string, 0, len(del.created))
		for _, dlv := range del.created {
			ids = append(ids, dlv.WebhookID)
		}
		return ids
	}

	// In-scope sector → both the scoped and the unscoped subscription receive it.
	if got := webhookIDs("s1"); len(got) != 2 {
		t.Errorf("sector s1: want 2 deliveries (scoped + all), got %d (%v)", len(got), got)
	}
	// Out-of-scope sector → only the unscoped subscription.
	if got := webhookIDs("s2"); len(got) != 1 || got[0] != "all" {
		t.Errorf("sector s2: want only the unscoped sub, got %v", got)
	}
	// No sector → only the unscoped subscription (scoped never matches "").
	if got := webhookIDs(""); len(got) != 1 || got[0] != "all" {
		t.Errorf("no sector: want only the unscoped sub, got %v", got)
	}
}

func TestDispatcher_UnsupportedEventIgnored(t *testing.T) {
	del := newFakeDeliveries()
	enq := &fakeEnqueuer{}
	d := NewDispatcher(&fakeSubs{byEvent: map[string][]*entity.WebhookSubscription{}}, del, enq, fixedClock{})
	d.Emit(ctxTenant(), "t1", "not.a.real.event", "", nil)
	if len(del.created) != 0 || len(enq.items) != 0 {
		t.Errorf("unsupported event should be a no-op")
	}
}

// ── delivery worker ──────────────────────────────────────────────────────────

func newDelivery(status entity.DeliveryStatus, attempts int) (*fakeDeliveries, *entity.WebhookDelivery) {
	del := newFakeDeliveries()
	d := &entity.WebhookDelivery{ID: "d1", TenantID: "t1", WebhookID: "wh1", Event: entity.EventMessageCreated, Payload: []byte(`{"x":1}`), Status: status, Attempts: attempts}
	del.created[d.ID] = d
	return del, d
}

func deliverySvc(subEnabled bool, del *fakeDeliveries, sender *fakeSender, enq *fakeEnqueuer, limiter contracts.RateLimiter) *DeliveryService {
	subs := &fakeSubs{byID: map[string]*entity.WebhookSubscription{
		"wh1": {ID: "wh1", TenantID: "t1", Enabled: subEnabled, Secret: "whsec_abc", URL: "https://x", RateLimitPerMin: 0},
	}}
	return NewDeliveryService(subs, del, sender, enq, limiter, fixedClock{t: time.Unix(1700000000, 0).UTC()})
}

func TestDeliver_Success(t *testing.T) {
	del, _ := newDelivery(entity.DeliveryPending, 0)
	sender := &fakeSender{status: 200}
	svc := deliverySvc(true, del, sender, &fakeEnqueuer{}, fakeLimiter{allow: true})
	if err := svc.Deliver(ctxTenant(), "d1"); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	got := del.created["d1"]
	if got.Status != entity.DeliveryDelivered || got.Attempts != 1 {
		t.Errorf("expected delivered with 1 attempt, got %+v", got)
	}
	if !sender.gotSig {
		t.Errorf("sender did not receive secret+payload for signing")
	}
}

func TestDeliver_FailureSchedulesRetryWithBackoff(t *testing.T) {
	del, _ := newDelivery(entity.DeliveryPending, 0)
	sender := &fakeSender{status: 500}
	enq := &fakeEnqueuer{}
	svc := deliverySvc(true, del, sender, enq, fakeLimiter{allow: true})
	if err := svc.Deliver(ctxTenant(), "d1"); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	got := del.created["d1"]
	if got.Status != entity.DeliveryRetrying || got.Attempts != 1 {
		t.Errorf("expected retrying with 1 attempt, got %+v", got)
	}
	if got.NextRetryAt == nil {
		t.Errorf("expected next_retry_at set")
	}
	if len(enq.items) != 1 || !enq.items[0].retry || enq.items[0].delay <= 0 {
		t.Errorf("expected a backoff retry enqueue, got %+v", enq.items)
	}
}

func TestDeliver_ExhaustionDeadLetters(t *testing.T) {
	// attempts already at max-1 so this attempt reaches the limit.
	del, _ := newDelivery(entity.DeliveryRetrying, defaultMaxAttempts-1)
	sender := &fakeSender{err: errors.New("connection refused")}
	enq := &fakeEnqueuer{}
	svc := deliverySvc(true, del, sender, enq, fakeLimiter{allow: true})
	if err := svc.Deliver(ctxTenant(), "d1"); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	got := del.created["d1"]
	if got.Status != entity.DeliveryDead {
		t.Errorf("expected dead-letter, got %+v", got)
	}
	if len(enq.items) != 0 {
		t.Errorf("dead delivery must not be re-enqueued, got %+v", enq.items)
	}
}

func TestDeliver_TerminalIsNoOp(t *testing.T) {
	del, _ := newDelivery(entity.DeliveryDelivered, 1)
	sender := &fakeSender{status: 200}
	svc := deliverySvc(true, del, sender, &fakeEnqueuer{}, fakeLimiter{allow: true})
	if err := svc.Deliver(ctxTenant(), "d1"); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if sender.calls != 0 {
		t.Errorf("terminal delivery should not call the sender")
	}
}

func TestDeliver_DisabledSubscriptionDeadLetters(t *testing.T) {
	del, _ := newDelivery(entity.DeliveryPending, 0)
	sender := &fakeSender{status: 200}
	svc := deliverySvc(false, del, sender, &fakeEnqueuer{}, fakeLimiter{allow: true})
	if err := svc.Deliver(ctxTenant(), "d1"); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if del.created["d1"].Status != entity.DeliveryDead {
		t.Errorf("disabled subscription should dead-letter, got %+v", del.created["d1"])
	}
	if sender.calls != 0 {
		t.Errorf("disabled subscription should not be sent")
	}
}

func TestDeliver_RateLimitedReschedulesWithoutAttempt(t *testing.T) {
	del, _ := newDelivery(entity.DeliveryPending, 0)
	sender := &fakeSender{status: 200}
	enq := &fakeEnqueuer{}
	svc := deliverySvc(true, del, sender, enq, fakeLimiter{allow: false})
	if err := svc.Deliver(ctxTenant(), "d1"); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	got := del.created["d1"]
	if got.Status != entity.DeliveryRetrying || got.Attempts != 0 {
		t.Errorf("rate-limited delivery should reschedule without consuming an attempt, got %+v", got)
	}
	if sender.calls != 0 {
		t.Errorf("rate-limited delivery should not be sent")
	}
	if len(enq.items) != 1 || !enq.items[0].retry {
		t.Errorf("expected a reschedule enqueue, got %+v", enq.items)
	}
}

// ── subscription service ─────────────────────────────────────────────────────

func TestSubscription_CreateValidatesEventsAndGeneratesSecret(t *testing.T) {
	subs := &fakeSubs{byID: map[string]*entity.WebhookSubscription{}}
	svc := NewSubscriptionService(subs, newFakeDeliveries(), &fakeSender{status: 200}, fixedClock{t: time.Unix(1700000000, 0).UTC()})

	// Unsupported event rejected.
	if _, err := svc.Create(ctxTenant(), contracts.CreateSubscription{URL: "https://x", Events: []string{"bogus"}}); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("expected validation error for bad event, got %v", err)
	}

	// Valid create generates a secret.
	sub, err := svc.Create(ctxTenant(), contracts.CreateSubscription{URL: "https://x", Events: []string{entity.EventMessageCreated}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if sub.Secret == "" {
		t.Errorf("expected a generated secret")
	}
}

func TestSubscription_TestRecordsDelivery(t *testing.T) {
	subs := &fakeSubs{byID: map[string]*entity.WebhookSubscription{
		"wh1": {ID: "wh1", TenantID: "t1", Enabled: true, Secret: "whsec_abc", URL: "https://x"},
	}}
	del := newFakeDeliveries()
	svc := NewSubscriptionService(subs, del, &fakeSender{status: 200}, fixedClock{t: time.Unix(1700000000, 0).UTC()})
	res, err := svc.Test(ctxTenant(), "wh1")
	if err != nil {
		t.Fatalf("test: %v", err)
	}
	if !res.OK || res.StatusCode != 200 {
		t.Errorf("expected ok test result, got %+v", res)
	}
	if len(del.created) != 1 {
		t.Errorf("expected the test delivery to be recorded")
	}
}

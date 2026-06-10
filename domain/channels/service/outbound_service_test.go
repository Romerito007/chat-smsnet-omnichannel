package service

import (
	"context"
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	chcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/contracts"
	chentity "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	contactentity "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/entity"
	convcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
)

type fakeDeliveryRepo struct {
	byID  map[string]*chentity.OutboundDelivery
	byExt map[string]*chentity.OutboundDelivery
}

func newFakeDeliveryRepo() *fakeDeliveryRepo {
	return &fakeDeliveryRepo{byID: map[string]*chentity.OutboundDelivery{}, byExt: map[string]*chentity.OutboundDelivery{}}
}
func (r *fakeDeliveryRepo) put(d *chentity.OutboundDelivery) {
	cp := *d
	r.byID[d.ID] = &cp
	if d.ExternalMessageID != "" {
		r.byExt[d.ExternalMessageID] = &cp
	}
}
func (r *fakeDeliveryRepo) Create(_ context.Context, d *chentity.OutboundDelivery) error {
	r.put(d)
	return nil
}
func (r *fakeDeliveryRepo) Update(_ context.Context, d *chentity.OutboundDelivery) error {
	r.put(d)
	return nil
}
func (r *fakeDeliveryRepo) FindByID(_ context.Context, id string) (*chentity.OutboundDelivery, error) {
	if d, ok := r.byID[id]; ok {
		cp := *d
		return &cp, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeDeliveryRepo) FindByExternalMessageID(_ context.Context, ext string) (*chentity.OutboundDelivery, error) {
	if d, ok := r.byExt[ext]; ok {
		cp := *d
		return &cp, nil
	}
	return nil, apperror.NotFound("nf")
}

type fakeEnqueuer struct {
	delivers int
	retries  int
}

func (e *fakeEnqueuer) EnqueueDeliver(chcontracts.DeliverTask) error    { e.delivers++; return nil }
func (e *fakeEnqueuer) EnqueueRetry(chcontracts.DeliverTask, int) error { e.retries++; return nil }

// ── fixture ──────────────────────────────────────────────────────────────────

type outboundFixture struct {
	svc        *OutboundService
	deliveries *fakeDeliveryRepo
	msgs       *fakeMsgRepo
	convs      *fakeConvRepo
	pub        *fakePublisher
	enq        *fakeEnqueuer
	adapter    *fakeAdapter
}

func newOutboundFixture() outboundFixture {
	conns := newFakeConnRepo()
	conns.put(&chentity.ChannelConnection{ID: "conn1", TenantID: "t1", Type: chentity.TypeCustom, Enabled: true})
	deliveries := newFakeDeliveryRepo()
	convs := newFakeConvRepo()
	convs.Create(context.Background(), &conventity.Conversation{ID: "conv1", TenantID: "t1", ContactID: "cont1", Channel: "custom", AssignedTo: "agent1"})
	msgs := newFakeMsgRepo()
	contacts := newFakeContactRepo()
	contacts.byID["cont1"] = &contactentity.Contact{ID: "cont1", TenantID: "t1", Phone: "5511", Identities: []contactentity.ChannelIdentity{{Channel: "custom", ExternalID: "ext-cont"}}}
	adapter := &fakeAdapter{}
	pub := &fakePublisher{}
	enq := &fakeEnqueuer{}
	svc := NewOutboundService(conns, deliveries, convs, msgs, contacts, fakeRegistry{adapter}, enq, pub, clockNow())
	return outboundFixture{svc: svc, deliveries: deliveries, msgs: msgs, convs: convs, pub: pub, enq: enq, adapter: adapter}
}

func seedMessage(fx outboundFixture, text string) (msgID, deliveryID string) {
	m := &conventity.Message{ID: "msg1", TenantID: "t1", ConversationID: "conv1", Direction: conventity.DirectionOutbound, Text: text, DeliveryStatus: conventity.DeliveryPending}
	fx.msgs.Create(context.Background(), m)
	d := &chentity.OutboundDelivery{ID: "del1", TenantID: "t1", ChannelConnectionID: "conn1", ConversationID: "conv1", MessageID: "msg1", Status: chentity.DeliveryPending}
	fx.deliveries.Create(context.Background(), d)
	return m.ID, d.ID
}

// ── tests ────────────────────────────────────────────────────────────────────

func TestDispatch_CreatesDeliveryAndEnqueues(t *testing.T) {
	fx := newOutboundFixture()
	conv := &conventity.Conversation{ID: "conv1", TenantID: "t1", Channel: "custom"}
	msg := &conventity.Message{ID: "m9", TenantID: "t1", ConversationID: "conv1", Direction: conventity.DirectionOutbound, Text: "hi"}

	fx.svc.Dispatch(tenantCtx(), conv, msg)

	if len(fx.deliveries.byID) != 1 {
		t.Errorf("expected one delivery created, got %d", len(fx.deliveries.byID))
	}
	if fx.enq.delivers != 1 {
		t.Errorf("expected one deliver enqueued, got %d", fx.enq.delivers)
	}
}

func TestDeliver_SuccessMarksSent(t *testing.T) {
	fx := newOutboundFixture()
	_, deliveryID := seedMessage(fx, "hello")

	if err := fx.svc.Deliver(tenantCtx(), deliveryID); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	d := fx.deliveries.byID[deliveryID]
	if d.Status != chentity.DeliverySent || d.ExternalMessageID == "" {
		t.Errorf("delivery should be sent with external id, got %+v", d)
	}
	m := fx.msgs.items["msg1"]
	if m.DeliveryStatus != conventity.DeliverySent || m.ExternalMessageID == "" {
		t.Errorf("message should be sent with external id, got %+v", m)
	}
	if !fx.pub.has(convcontracts.RealtimeMessageSent) {
		t.Error("expected message.sent realtime event")
	}
}

func TestDeliver_RetriesThenFails(t *testing.T) {
	fx := newOutboundFixture()
	fx.adapter.failSend = true
	_, deliveryID := seedMessage(fx, "hello")

	// Simulate the deliver + retry jobs firing until the limit is reached.
	for i := 0; i < defaultMaxAttempts; i++ {
		if err := fx.svc.Deliver(tenantCtx(), deliveryID); err != nil {
			t.Fatalf("deliver attempt %d: %v", i, err)
		}
	}

	d := fx.deliveries.byID[deliveryID]
	if d.Status != chentity.DeliveryFailed {
		t.Errorf("delivery should be failed after max attempts, got %s (attempts %d)", d.Status, d.Attempts)
	}
	m := fx.msgs.items["msg1"]
	if m.DeliveryStatus != conventity.DeliveryFailed {
		t.Errorf("message should be failed, got %s", m.DeliveryStatus)
	}
	if fx.enq.retries != defaultMaxAttempts-1 {
		t.Errorf("expected %d retries scheduled, got %d", defaultMaxAttempts-1, fx.enq.retries)
	}
	if !fx.pub.has(convcontracts.RealtimeMessageFailed) {
		t.Error("expected message.failed realtime event")
	}
}

func TestReceipt_AdvancesAndIsIdempotent(t *testing.T) {
	fx := newOutboundFixture()
	_, deliveryID := seedMessage(fx, "hello")
	if err := fx.svc.Deliver(tenantCtx(), deliveryID); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	ext := fx.deliveries.byID[deliveryID].ExternalMessageID

	// delivered
	if err := fx.svc.ApplyReceipt(tenantCtx(), chcontracts.DeliveryReceipt{ExternalMessageID: ext, Status: chentity.DeliveryDelivered}); err != nil {
		t.Fatalf("receipt delivered: %v", err)
	}
	if fx.msgs.items["msg1"].DeliveryStatus != conventity.DeliveryDelivered {
		t.Errorf("message should be delivered")
	}

	// duplicate delivered → no-op (still delivered, not regressed/duplicated)
	if err := fx.svc.ApplyReceipt(tenantCtx(), chcontracts.DeliveryReceipt{ExternalMessageID: ext, Status: chentity.DeliveryDelivered}); err != nil {
		t.Fatalf("duplicate receipt: %v", err)
	}
	if fx.deliveries.byExt[ext].Status != chentity.DeliveryDelivered {
		t.Errorf("duplicate receipt changed status unexpectedly")
	}

	// read advances
	if err := fx.svc.ApplyReceipt(tenantCtx(), chcontracts.DeliveryReceipt{ExternalMessageID: ext, Status: chentity.DeliveryRead}); err != nil {
		t.Fatalf("receipt read: %v", err)
	}
	if fx.msgs.items["msg1"].DeliveryStatus != conventity.DeliveryRead {
		t.Errorf("message should be read")
	}

	// out-of-order/older receipt (delivered after read) → ignored
	if err := fx.svc.ApplyReceipt(tenantCtx(), chcontracts.DeliveryReceipt{ExternalMessageID: ext, Status: chentity.DeliveryDelivered}); err != nil {
		t.Fatalf("stale receipt: %v", err)
	}
	if fx.deliveries.byExt[ext].Status != chentity.DeliveryRead {
		t.Errorf("stale receipt should not regress status, got %s", fx.deliveries.byExt[ext].Status)
	}
}

func TestReceipt_UnknownExternalIDIsNoop(t *testing.T) {
	fx := newOutboundFixture()
	if err := fx.svc.ApplyReceipt(tenantCtx(), chcontracts.DeliveryReceipt{ExternalMessageID: "ghost", Status: chentity.DeliveryDelivered}); err != nil {
		t.Errorf("unknown external id should be a no-op, got %v", err)
	}
}

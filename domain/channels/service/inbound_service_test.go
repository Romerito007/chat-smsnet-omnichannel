package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	chcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/contracts"
	chentity "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	contactentity "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/entity"
	contactrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/repository"
	contactservice "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/service"
	convcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	queueentity "github.com/romerito007/chat-smsnet-omnichannel/domain/queues/entity"
	queuerepo "github.com/romerito007/chat-smsnet-omnichannel/domain/queues/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// ── doubles ──────────────────────────────────────────────────────────────────

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type fakeContactRepo struct {
	contactrepo.ContactRepository
	byIdentity map[string]*contactentity.Contact // channel|external -> contact
	byID       map[string]*contactentity.Contact
}

func newFakeContactRepo() *fakeContactRepo {
	return &fakeContactRepo{byIdentity: map[string]*contactentity.Contact{}, byID: map[string]*contactentity.Contact{}}
}
func idKey(ch, ext string) string { return ch + "|" + ext }

func (r *fakeContactRepo) Create(_ context.Context, c *contactentity.Contact) error {
	cp := *c
	r.byID[c.ID] = &cp
	for _, id := range c.Identities {
		r.byIdentity[idKey(id.Channel, id.ExternalID)] = &cp
	}
	return nil
}
func (r *fakeContactRepo) Update(_ context.Context, c *contactentity.Contact) error {
	cp := *c
	r.byID[c.ID] = &cp
	for _, id := range c.Identities {
		r.byIdentity[idKey(id.Channel, id.ExternalID)] = &cp
	}
	return nil
}
func (r *fakeContactRepo) FindByID(_ context.Context, id string) (*contactentity.Contact, error) {
	if c, ok := r.byID[id]; ok {
		return c, nil
	}
	return nil, apperror.NotFound("not found")
}
func (r *fakeContactRepo) FindByChannelIdentity(_ context.Context, ch, ext string) (*contactentity.Contact, error) {
	if c, ok := r.byIdentity[idKey(ch, ext)]; ok {
		return c, nil
	}
	return nil, apperror.NotFound("not found")
}

type fakeConvRepo struct {
	mu    sync.Mutex
	items map[string]*conventity.Conversation
}

func (r *fakeConvRepo) Create(_ context.Context, c *conventity.Conversation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *c
	r.items[c.ID] = &cp
	return nil
}
func (r *fakeConvRepo) Update(_ context.Context, c *conventity.Conversation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *c
	r.items[c.ID] = &cp
	return nil
}
func (r *fakeConvRepo) FindByID(_ context.Context, id string) (*conventity.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.items[id]; ok {
		cp := *c
		return &cp, nil
	}
	return nil, apperror.NotFound("not found")
}
func (r *fakeConvRepo) FindOpenByContactChannel(_ context.Context, contactID, channel string) (*conventity.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.items {
		if c.ContactID == contactID && c.Channel == channel && !c.Status.IsClosed() {
			cp := *c
			return &cp, nil
		}
	}
	return nil, apperror.NotFound("not found")
}
func (r *fakeConvRepo) List(context.Context, convcontracts.ListFilter, convcontracts.Visibility, shared.PageRequest) ([]*conventity.Conversation, error) {
	return nil, nil
}

type fakeMsgRepo struct {
	mu    sync.Mutex
	items []*conventity.Message
}

func (r *fakeMsgRepo) Create(_ context.Context, m *conventity.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items = append(r.items, m)
	return nil
}
func (r *fakeMsgRepo) Update(context.Context, *conventity.Message) error { return nil }
func (r *fakeMsgRepo) FindByID(context.Context, string) (*conventity.Message, error) {
	return nil, apperror.NotFound("nf")
}
func (r *fakeMsgRepo) ListByConversation(context.Context, string, shared.PageRequest) ([]*conventity.Message, error) {
	return r.items, nil
}
func (r *fakeMsgRepo) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.items)
}

type fakeEventRepo struct {
	mu    sync.Mutex
	items []*conventity.ConversationEvent
}

func (r *fakeEventRepo) Create(_ context.Context, e *conventity.ConversationEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items = append(r.items, e)
	return nil
}
func (r *fakeEventRepo) ListByConversation(context.Context, string, shared.PageRequest) ([]*conventity.ConversationEvent, error) {
	return r.items, nil
}

type fakeQueues struct {
	queuerepo.QueueRepository
	byID map[string]*queueentity.Queue
}

func (r *fakeQueues) FindByID(_ context.Context, id string) (*queueentity.Queue, error) {
	if q, ok := r.byID[id]; ok {
		return q, nil
	}
	return nil, apperror.NotFound("not found")
}

// fakeInbound is the idempotency ledger with a real uniqueness constraint.
type fakeInbound struct {
	mu      sync.Mutex
	records map[string]*chentity.InboundRecord // tenant|channel|external -> record
}

func newFakeInbound() *fakeInbound {
	return &fakeInbound{records: map[string]*chentity.InboundRecord{}}
}
func recKey(tenant, ch, ext string) string { return tenant + "|" + ch + "|" + ext }

func (r *fakeInbound) FindByExternalID(ctx context.Context, channel, ext string) (*chentity.InboundRecord, error) {
	tenant, _ := shared.TenantFrom(ctx)
	r.mu.Lock()
	defer r.mu.Unlock()
	if rec, ok := r.records[recKey(tenant, channel, ext)]; ok {
		return rec, nil
	}
	return nil, apperror.NotFound("not found")
}
func (r *fakeInbound) Create(ctx context.Context, rec *chentity.InboundRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := recKey(rec.TenantID, rec.Channel, rec.ExternalMessageID)
	if _, ok := r.records[k]; ok {
		return apperror.Conflict("duplicate")
	}
	r.records[k] = rec
	return nil
}

type capturedEvent struct{ topic, event string }
type fakePublisher struct {
	mu     sync.Mutex
	events []capturedEvent
}

func (p *fakePublisher) Publish(_ context.Context, topic, event string, _ any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, capturedEvent{topic, event})
	return nil
}
func (p *fakePublisher) has(event string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, e := range p.events {
		if e.event == event {
			return true
		}
	}
	return false
}

type fakeDispatcher struct {
	mu    sync.Mutex
	calls int
}

func (d *fakeDispatcher) Dispatch(chcontracts.AutomationInvoke) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls++
	return nil
}

// ── fixture ──────────────────────────────────────────────────────────────────

type fixture struct {
	svc      *InboundService
	convs    *fakeConvRepo
	msgs     *fakeMsgRepo
	pub      *fakePublisher
	ledger   *fakeInbound
	dispatch *fakeDispatcher
}

func newFixture(queues map[string]*queueentity.Queue) fixture {
	cr := &fakeConvRepo{items: map[string]*conventity.Conversation{}}
	mr := &fakeMsgRepo{}
	er := &fakeEventRepo{}
	pub := &fakePublisher{}
	ledger := newFakeInbound()
	dispatch := &fakeDispatcher{}
	contacts := contactservice.New(newFakeContactRepo(), fixedClock{t: time.Unix(1700000000, 0).UTC()})
	svc := NewInboundService(contacts, cr, mr, er, &fakeQueues{byID: queues}, ledger, dispatch,
		shared.NoopLocker{}, pub, fixedClock{t: time.Unix(1700000000, 0).UTC()})
	return fixture{svc: svc, convs: cr, msgs: mr, pub: pub, ledger: ledger, dispatch: dispatch}
}

func tenantCtx() context.Context { return shared.WithTenant(context.Background(), "t1") }

func integ(automation bool, queueID string) *chentity.Integration {
	return &chentity.Integration{ID: "i1", TenantID: "t1", Channel: "whatsapp", Enabled: true, AutomationEnabled: automation, DefaultQueueID: queueID}
}

func inbound(ext string) chcontracts.InboundMessage {
	return chcontracts.InboundMessage{
		IntegrationKey:    "k",
		ExternalMessageID: ext,
		ExternalContactID: "5511999",
		ContactName:       "Jane",
		ContactPhone:      "+55 11 99999",
		Channel:           "whatsapp",
		Text:              "hello",
	}
}

// ── tests ────────────────────────────────────────────────────────────────────

func TestInbound_CreatesContactConversationMessageAndQueues(t *testing.T) {
	fx := newFixture(map[string]*queueentity.Queue{"q1": {ID: "q1", TenantID: "t1", SectorID: "s1"}})
	res, err := fx.svc.Handle(tenantCtx(), integ(false, "q1"), inbound("ext-1"))
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if res.ConversationID == "" || res.MessageID == "" || res.ContactID == "" {
		t.Fatalf("missing ids in result: %+v", res)
	}
	if res.Status != string(conventity.StatusQueued) {
		t.Errorf("status = %q, want queued", res.Status)
	}
	conv := fx.convs.items[res.ConversationID]
	if conv.QueueID != "q1" || conv.SectorID != "s1" {
		t.Errorf("queue/sector not set from default queue: %+v", conv)
	}
	if fx.msgs.count() != 1 {
		t.Errorf("expected 1 message, got %d", fx.msgs.count())
	}
	if !fx.pub.has(convcontracts.RealtimeMessageCreated) || !fx.pub.has(convcontracts.RealtimeConversationUpdated) {
		t.Error("expected message.created and conversation.updated realtime events")
	}
}

func TestInbound_NewConversationWithAutomation(t *testing.T) {
	fx := newFixture(nil)
	res, err := fx.svc.Handle(tenantCtx(), integ(true, ""), inbound("ext-2"))
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if res.Status != string(conventity.StatusAutomation) {
		t.Errorf("status = %q, want automation", res.Status)
	}
	if fx.dispatch.calls != 1 {
		t.Errorf("expected automation dispatch once, got %d", fx.dispatch.calls)
	}
}

func TestInbound_IdempotentResend(t *testing.T) {
	fx := newFixture(map[string]*queueentity.Queue{"q1": {ID: "q1", TenantID: "t1", SectorID: "s1"}})
	ctx := tenantCtx()
	first, err := fx.svc.Handle(ctx, integ(false, "q1"), inbound("dup-1"))
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := fx.svc.Handle(ctx, integ(false, "q1"), inbound("dup-1"))
	if err != nil {
		t.Fatalf("second: %v", err)
	}

	if !second.Idempotent {
		t.Error("resend should be flagged idempotent")
	}
	if second.ConversationID != first.ConversationID || second.MessageID != first.MessageID {
		t.Errorf("resend should map to the same conversation/message")
	}
	if fx.msgs.count() != 1 {
		t.Errorf("resend must not create a duplicate message, got %d", fx.msgs.count())
	}
	if len(fx.convs.items) != 1 {
		t.Errorf("resend must not create a duplicate conversation, got %d", len(fx.convs.items))
	}
}

func TestInbound_SecondMessageReusesOpenConversation(t *testing.T) {
	fx := newFixture(map[string]*queueentity.Queue{"q1": {ID: "q1", TenantID: "t1", SectorID: "s1"}})
	ctx := tenantCtx()
	first, _ := fx.svc.Handle(ctx, integ(false, "q1"), inbound("m-1"))
	second, err := fx.svc.Handle(ctx, integ(false, "q1"), inbound("m-2"))
	if err != nil {
		t.Fatalf("second message: %v", err)
	}
	if second.ConversationID != first.ConversationID {
		t.Errorf("a new inbound message should reuse the open conversation")
	}
	if fx.msgs.count() != 2 {
		t.Errorf("expected 2 messages, got %d", fx.msgs.count())
	}
	if len(fx.convs.items) != 1 {
		t.Errorf("expected 1 conversation, got %d", len(fx.convs.items))
	}
}

func TestInbound_RequiresExternalMessageID(t *testing.T) {
	fx := newFixture(nil)
	msg := inbound("")
	if _, err := fx.svc.Handle(tenantCtx(), integ(false, ""), msg); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("expected validation_error, got %v", err)
	}
}

func TestInbound_MessageIsInbound(t *testing.T) {
	fx := newFixture(nil)
	if _, err := fx.svc.Handle(tenantCtx(), integ(false, ""), inbound("x1")); err != nil {
		t.Fatalf("handle: %v", err)
	}
	m := fx.msgs.items[0]
	if m.Direction != conventity.DirectionInbound || m.SenderType != conventity.SenderCustomer {
		t.Errorf("inbound message should be inbound/customer, got %s/%s", m.Direction, m.SenderType)
	}
	if m.ExternalMessageID != "x1" {
		t.Errorf("external id not persisted: %q", m.ExternalMessageID)
	}
}

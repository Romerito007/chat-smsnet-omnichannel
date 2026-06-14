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
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// ── shared conversation/contact/message/event/ledger fakes ───────────────────

type fakeContactRepo struct {
	contactrepo.ContactRepository
	byIdentity map[string]*contactentity.Contact
	byID       map[string]*contactentity.Contact
}

func newFakeContactRepo() *fakeContactRepo {
	return &fakeContactRepo{byIdentity: map[string]*contactentity.Contact{}, byID: map[string]*contactentity.Contact{}}
}
func cidKey(ch, ext string) string { return ch + "|" + ext }

func (r *fakeContactRepo) Create(_ context.Context, c *contactentity.Contact) error {
	cp := *c
	r.byID[c.ID] = &cp
	for _, id := range c.Identities {
		r.byIdentity[cidKey(id.Channel, id.ExternalID)] = &cp
	}
	return nil
}
func (r *fakeContactRepo) Update(_ context.Context, c *contactentity.Contact) error {
	return r.Create(context.Background(), c)
}
func (r *fakeContactRepo) FindByID(_ context.Context, id string) (*contactentity.Contact, error) {
	if c, ok := r.byID[id]; ok {
		return c, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeContactRepo) FindByChannelIdentity(_ context.Context, ch, ext string) (*contactentity.Contact, error) {
	if c, ok := r.byIdentity[cidKey(ch, ext)]; ok {
		return c, nil
	}
	return nil, apperror.NotFound("nf")
}

type fakeConvRepo struct {
	mu    sync.Mutex
	items map[string]*conventity.Conversation
}

func newFakeConvRepo() *fakeConvRepo {
	return &fakeConvRepo{items: map[string]*conventity.Conversation{}}
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
	return nil, apperror.NotFound("nf")
}
func (r *fakeConvRepo) FindByIDs(context.Context, []string) ([]*conventity.Conversation, error) {
	return nil, nil
}
func (r *fakeConvRepo) FindOpenByContactChannelID(_ context.Context, contactID, channelID string) (*conventity.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.items {
		if c.ContactID == contactID && c.ChannelID == channelID && !c.Status.IsClosed() {
			cp := *c
			return &cp, nil
		}
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeConvRepo) ListInactiveOpen(context.Context, time.Time, int) ([]*conventity.Conversation, error) {
	return nil, nil
}
func (r *fakeConvRepo) List(context.Context, convcontracts.ListFilter, convcontracts.Visibility, shared.PageRequest) ([]*conventity.Conversation, error) {
	return nil, nil
}

type fakeMsgRepo struct {
	mu    sync.Mutex
	items map[string]*conventity.Message
	order []string
}

func newFakeMsgRepo() *fakeMsgRepo {
	return &fakeMsgRepo{items: map[string]*conventity.Message{}}
}
func (r *fakeMsgRepo) Create(_ context.Context, m *conventity.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *m
	r.items[m.ID] = &cp
	r.order = append(r.order, m.ID)
	return nil
}
func (r *fakeMsgRepo) Update(_ context.Context, m *conventity.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *m
	r.items[m.ID] = &cp
	return nil
}
func (r *fakeMsgRepo) FindByID(_ context.Context, id string) (*conventity.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if m, ok := r.items[id]; ok {
		cp := *m
		return &cp, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeMsgRepo) ListByConversation(context.Context, string, shared.PageRequest) ([]*conventity.Message, error) {
	return nil, nil
}
func (r *fakeMsgRepo) LatestByConversation(context.Context, string) (*conventity.Message, error) {
	return nil, apperror.NotFound("none")
}
func (r *fakeMsgRepo) LatestByConversations(context.Context, []string) (map[string]*conventity.Message, error) {
	return nil, nil
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

type fakeInbound struct {
	mu      sync.Mutex
	records map[string]*chentity.InboundRecord
}

func newFakeInbound() *fakeInbound {
	return &fakeInbound{records: map[string]*chentity.InboundRecord{}}
}
func recKey(t, ch, ext string) string { return t + "|" + ch + "|" + ext }

func (r *fakeInbound) FindByExternalID(ctx context.Context, channel, ext string) (*chentity.InboundRecord, error) {
	tenant, _ := shared.TenantFrom(ctx)
	r.mu.Lock()
	defer r.mu.Unlock()
	if rec, ok := r.records[recKey(tenant, channel, ext)]; ok {
		return rec, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeInbound) Create(_ context.Context, rec *chentity.InboundRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := recKey(rec.TenantID, rec.Channel, rec.ExternalMessageID)
	if _, ok := r.records[k]; ok {
		return apperror.Conflict("dup")
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

type fakeDispatcher struct{ calls int }

func (d *fakeDispatcher) Dispatch(chcontracts.AutomationInvoke) error { d.calls++; return nil }

// ── inbound fixture/tests ────────────────────────────────────────────────────

type inboundFixture struct {
	svc      *InboundService
	convs    *fakeConvRepo
	msgs     *fakeMsgRepo
	pub      *fakePublisher
	dispatch *fakeDispatcher
}

func newInboundFixture() inboundFixture {
	cr := newFakeConvRepo()
	mr := newFakeMsgRepo()
	pub := &fakePublisher{}
	dispatch := &fakeDispatcher{}
	contacts := contactservice.New(newFakeContactRepo(), clockNow())
	svc := NewInboundService(contacts, cr, mr, &fakeEventRepo{}, newFakeInbound(), dispatch,
		shared.NoopLocker{}, pub, clockNow())
	return inboundFixture{svc: svc, convs: cr, msgs: mr, pub: pub, dispatch: dispatch}
}

func conn(automation bool, sector string) *chentity.ChannelConnection {
	return &chentity.ChannelConnection{
		ID: "conn1", TenantID: "t1", Type: chentity.TypeWhatsApp, Enabled: true,
		AutomationEnabled: automation, DefaultSectorID: sector,
	}
}

func inMsg(ext string) chcontracts.InboundMessage {
	return chcontracts.InboundMessage{
		ExternalMessageID: ext, ExternalContactID: "5511999", ContactName: "Jane",
		Channel: "whatsapp", Text: "hello",
	}
}

func TestInbound_CreatesAndQueuesIntoDefaultSector(t *testing.T) {
	fx := newInboundFixture()
	res, err := fx.svc.Handle(tenantCtx(), conn(false, "sector-x"), inMsg("ext-1"))
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if res.Status != string(conventity.StatusQueued) {
		t.Errorf("status = %q, want queued", res.Status)
	}
	if fx.convs.items[res.ConversationID].SectorID != "sector-x" {
		t.Errorf("expected default sector set")
	}
	// The conversation must persist the SPECIFIC channel connection id (conn1),
	// not just the type — this is what makes per-channel routing/hours/assistant work.
	if got := fx.convs.items[res.ConversationID].ChannelID; got != "conn1" {
		t.Errorf("conversation channel_id = %q, want conn1 (the connection id)", got)
	}
	if !fx.pub.has(convcontracts.RealtimeMessageCreated) || !fx.pub.has(convcontracts.RealtimeConversationUpdated) {
		t.Error("expected realtime events")
	}
	if fx.convs.items[res.ConversationID].UnreadCount != 1 {
		t.Errorf("inbound message should bump unread_count to 1, got %d", fx.convs.items[res.ConversationID].UnreadCount)
	}
}

func TestInbound_AutomationChannel(t *testing.T) {
	fx := newInboundFixture()
	res, err := fx.svc.Handle(tenantCtx(), conn(true, ""), inMsg("ext-2"))
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if res.Status != string(conventity.StatusAutomation) {
		t.Errorf("status = %q, want automation", res.Status)
	}
	if fx.dispatch.calls != 1 {
		t.Errorf("expected one automation dispatch, got %d", fx.dispatch.calls)
	}
}

func TestInbound_IdempotentResend(t *testing.T) {
	fx := newInboundFixture()
	ctx := tenantCtx()
	first, _ := fx.svc.Handle(ctx, conn(false, "s"), inMsg("dup"))
	second, err := fx.svc.Handle(ctx, conn(false, "s"), inMsg("dup"))
	if err != nil {
		t.Fatalf("resend: %v", err)
	}
	if !second.Idempotent || second.MessageID != first.MessageID {
		t.Errorf("resend should be idempotent to the same message")
	}
	if fx.msgs.count() != 1 || len(fx.convs.items) != 1 {
		t.Errorf("resend must not duplicate (messages=%d convs=%d)", fx.msgs.count(), len(fx.convs.items))
	}
}

func TestInbound_ReusesOpenConversation(t *testing.T) {
	fx := newInboundFixture()
	ctx := tenantCtx()
	first, _ := fx.svc.Handle(ctx, conn(false, "s"), inMsg("m1"))
	second, _ := fx.svc.Handle(ctx, conn(false, "s"), inMsg("m2"))
	if second.ConversationID != first.ConversationID {
		t.Error("second message should reuse the open conversation")
	}
	if fx.msgs.count() != 2 {
		t.Errorf("expected 2 messages, got %d", fx.msgs.count())
	}
}

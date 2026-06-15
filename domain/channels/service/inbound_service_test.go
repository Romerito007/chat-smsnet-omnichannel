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
func (r *fakeConvRepo) FindLastByContactChannelID(_ context.Context, contactID, channelID string) (*conventity.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var last *conventity.Conversation
	for _, c := range r.items {
		if c.ContactID == contactID && c.ChannelID == channelID {
			if last == nil || c.UpdatedAt.After(last.UpdatedAt) {
				last = c
			}
		}
	}
	if last == nil {
		return nil, apperror.NotFound("nf")
	}
	cp := *last
	return &cp, nil
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

type capturedEvent struct {
	topic, event string
	payload      any
}
type fakePublisher struct {
	mu     sync.Mutex
	events []capturedEvent
}

func (p *fakePublisher) Publish(_ context.Context, topic, event string, payload any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, capturedEvent{topic, event, payload})
	return nil
}

// lastPayload returns the payload of the most recent event with the given name.
func (p *fakePublisher) lastPayload(event string) any {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := len(p.events) - 1; i >= 0; i-- {
		if p.events[i].event == event {
			return p.events[i].payload
		}
	}
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

// ── inbound fixture/tests ────────────────────────────────────────────────────

type inboundFixture struct {
	svc    *InboundService
	convs  *fakeConvRepo
	msgs   *fakeMsgRepo
	events *fakeEventRepo
	rules  *fakeRuleSink
	pub    *fakePublisher
}

// fakeProtocolCounter hands out 1,2,3,... per call (year ignored — the test only
// checks the format and monotonicity).
type fakeProtocolCounter struct {
	mu  sync.Mutex
	seq int64
}

func (c *fakeProtocolCounter) NextSequence(context.Context, string, int) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.seq++
	return c.seq, nil
}

// fakeRuleSink records emitted automation-rule events.
type fakeRuleSink struct {
	mu     sync.Mutex
	events []string
}

func (s *fakeRuleSink) EmitRuleEvent(_ context.Context, _, event, _ string, _ any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
}
func (s *fakeRuleSink) has(event string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.events {
		if e == event {
			return true
		}
	}
	return false
}

func newInboundFixture() inboundFixture {
	cr := newFakeConvRepo()
	mr := newFakeMsgRepo()
	er := &fakeEventRepo{}
	rules := &fakeRuleSink{}
	pub := &fakePublisher{}
	contacts := contactservice.New(newFakeContactRepo(), clockNow())
	svc := NewInboundService(contacts, cr, mr, er, &fakeProtocolCounter{}, newFakeInbound(),
		shared.NoopLocker{}, pub, clockNow())
	svc.SetRuleSink(rules)
	return inboundFixture{svc: svc, convs: cr, msgs: mr, events: er, rules: rules, pub: pub}
}

func conn(_ string) *chentity.ChannelConnection {
	return &chentity.ChannelConnection{
		ID: "conn1", TenantID: "t1", Type: chentity.TypeWhatsApp, Enabled: true,
	}
}

func inMsg(ext string) chcontracts.InboundMessage {
	return chcontracts.InboundMessage{
		ExternalMessageID: ext, ExternalContactID: "5511999", ContactName: "Jane",
		Channel: "whatsapp", Text: "hello",
	}
}

func TestInbound_CreatesAndQueuesWithoutSector(t *testing.T) {
	fx := newInboundFixture()
	res, err := fx.svc.Handle(tenantCtx(), conn(""), inMsg("ext-1"))
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if res.Status != string(conventity.StatusQueued) {
		t.Errorf("status = %q, want queued", res.Status)
	}
	// The channel no longer carries a sector: a new conversation is queued WITHOUT
	// one, awaiting assignment (Chatwoot model).
	if fx.convs.items[res.ConversationID].SectorID != "" {
		t.Errorf("expected no sector on a channel-less-sector inbound, got %q", fx.convs.items[res.ConversationID].SectorID)
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

// TestInbound_DenormalizesLastMessage verifies the inbound chokepoint writes the
// last-message snapshot onto the conversation (what the inbox reads), so an inbound
// message updates the inbox preview without any aggregation.
func TestInbound_DenormalizesLastMessage(t *testing.T) {
	fx := newInboundFixture()
	res, err := fx.svc.Handle(tenantCtx(), conn(""), inMsg("ext-snap"))
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	conv := fx.convs.items[res.ConversationID]
	if conv.LastMessage == nil {
		t.Fatal("inbound must write the denormalized last_message snapshot")
	}
	if conv.LastMessage.Preview != "hello" || conv.LastMessage.SenderType != conventity.SenderCustomer {
		t.Errorf("snapshot mismatch: %+v", conv.LastMessage)
	}
}

func TestInbound_IdempotentResend(t *testing.T) {
	fx := newInboundFixture()
	ctx := tenantCtx()
	first, _ := fx.svc.Handle(ctx, conn("s"), inMsg("dup"))
	second, err := fx.svc.Handle(ctx, conn("s"), inMsg("dup"))
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
	first, _ := fx.svc.Handle(ctx, conn("s"), inMsg("m1"))
	second, _ := fx.svc.Handle(ctx, conn("s"), inMsg("m2"))
	if second.ConversationID != first.ConversationID {
		t.Error("second message should reuse the open conversation")
	}
	if fx.msgs.count() != 2 {
		t.Errorf("expected 2 messages, got %d", fx.msgs.count())
	}
}

// protocolConn is a channel connection with protocol numbering enabled.
func protocolConn(sector string) *chentity.ChannelConnection {
	c := conn(sector)
	c.UsesProtocol = true
	return c
}

// closeConv marks a stored conversation closed (as a human/job would).
func (fx inboundFixture) closeConv(id string) {
	c := fx.convs.items[id]
	c.Status = conventity.StatusClosed
	now := c.UpdatedAt.Add(time.Minute)
	c.ClosedAt = &now
}

// SINGLE MODE: a closed last conversation is REOPENED (same conversation), not a
// new one — and no protocol is assigned. This is the new behavior.
func TestInbound_SingleMode_ReopensClosedConversation(t *testing.T) {
	fx := newInboundFixture()
	ctx := tenantCtx()

	first, err := fx.svc.Handle(ctx, conn("s"), inMsg("m1"))
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	fx.closeConv(first.ConversationID)

	second, err := fx.svc.Handle(ctx, conn("s"), inMsg("m2"))
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if second.ConversationID != first.ConversationID {
		t.Errorf("single mode must REOPEN the same conversation, got new %s", second.ConversationID)
	}
	if len(fx.convs.items) != 1 {
		t.Errorf("single mode must not create a second conversation, have %d", len(fx.convs.items))
	}
	reopened := fx.convs.items[first.ConversationID]
	if reopened.Status.IsClosed() || reopened.ClosedAt != nil {
		t.Errorf("reopened conversation must be open with closed_at cleared, got status=%s", reopened.Status)
	}
	if reopened.Protocol != "" {
		t.Errorf("single mode must not assign a protocol, got %q", reopened.Protocol)
	}
	if !fx.rules.has(string(conventity.EventConversationReopened)) {
		t.Errorf("reopen must emit the conversation.reopened rule event")
	}
}

// PROTOCOL MODE: a closed last conversation yields a NEW conversation with a NEW
// protocol; an OPEN one is reused (keeps its protocol).
func TestInbound_ProtocolMode_NewProtocolWhenClosed(t *testing.T) {
	fx := newInboundFixture()
	ctx := tenantCtx()

	first, err := fx.svc.Handle(ctx, protocolConn("s"), inMsg("m1"))
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	p1 := fx.convs.items[first.ConversationID].Protocol
	if p1 == "" {
		t.Fatalf("protocol mode must assign a protocol on the first conversation")
	}

	// Same OPEN conversation is reused, keeping its protocol.
	reuse, _ := fx.svc.Handle(ctx, protocolConn("s"), inMsg("m2"))
	if reuse.ConversationID != first.ConversationID {
		t.Errorf("open conversation must be reused in protocol mode too")
	}
	if fx.convs.items[reuse.ConversationID].Protocol != p1 {
		t.Errorf("reused conversation must keep its protocol")
	}

	// Close it; the next inbound creates a NEW conversation with a NEW protocol.
	fx.closeConv(first.ConversationID)
	third, _ := fx.svc.Handle(ctx, protocolConn("s"), inMsg("m3"))
	if third.ConversationID == first.ConversationID {
		t.Errorf("protocol mode must create a NEW conversation after close, not reopen")
	}
	p2 := fx.convs.items[third.ConversationID].Protocol
	if p2 == "" || p2 == p1 {
		t.Errorf("new conversation must get a fresh distinct protocol, got %q (first %q)", p2, p1)
	}
	if len(fx.convs.items) != 2 {
		t.Errorf("expected 2 conversations (closed + new), have %d", len(fx.convs.items))
	}
}

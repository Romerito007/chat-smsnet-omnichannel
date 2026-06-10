package service

import (
	"context"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	sectorentity "github.com/romerito007/chat-smsnet-omnichannel/domain/sectors/entity"
	sectorrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/sectors/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// ── doubles ──────────────────────────────────────────────────────────────────

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type fakeConvRepo struct {
	items map[string]*entity.Conversation
}

func (r *fakeConvRepo) Create(_ context.Context, c *entity.Conversation) error {
	cp := *c
	r.items[c.ID] = &cp
	return nil
}
func (r *fakeConvRepo) Update(_ context.Context, c *entity.Conversation) error {
	if _, ok := r.items[c.ID]; !ok {
		return apperror.NotFound("not found")
	}
	cp := *c
	r.items[c.ID] = &cp
	return nil
}
func (r *fakeConvRepo) FindByID(ctx context.Context, id string) (*entity.Conversation, error) {
	tenant, _ := shared.TenantFrom(ctx)
	if c, ok := r.items[id]; ok && c.TenantID == tenant {
		cp := *c
		return &cp, nil
	}
	return nil, apperror.NotFound("not found")
}
func (r *fakeConvRepo) List(ctx context.Context, f contracts.ListFilter, vis contracts.Visibility, _ shared.PageRequest) ([]*entity.Conversation, error) {
	tenant, _ := shared.TenantFrom(ctx)
	var out []*entity.Conversation
	for _, c := range r.items {
		if c.TenantID != tenant {
			continue
		}
		if f.Status != "" && string(c.Status) != f.Status {
			continue
		}
		if !vis.All {
			visible := c.AssignedTo == vis.UserID
			for _, s := range vis.SectorIDs {
				if s == c.SectorID && s != "" {
					visible = true
				}
			}
			if !visible {
				continue
			}
		}
		cp := *c
		out = append(out, &cp)
	}
	return out, nil
}

type fakeMsgRepo struct{ items []*entity.Message }

func (r *fakeMsgRepo) Create(_ context.Context, m *entity.Message) error {
	cp := *m
	r.items = append(r.items, &cp)
	return nil
}
func (r *fakeMsgRepo) Update(_ context.Context, _ *entity.Message) error { return nil }
func (r *fakeMsgRepo) FindByID(_ context.Context, id string) (*entity.Message, error) {
	for _, m := range r.items {
		if m.ID == id {
			return m, nil
		}
	}
	return nil, apperror.NotFound("not found")
}
func (r *fakeMsgRepo) ListByConversation(_ context.Context, convID string, _ shared.PageRequest) ([]*entity.Message, error) {
	var out []*entity.Message
	for _, m := range r.items {
		if m.ConversationID == convID && m.DeletedAt == nil {
			out = append(out, m)
		}
	}
	return out, nil
}

type fakeEventRepo struct{ items []*entity.ConversationEvent }

func (r *fakeEventRepo) Create(_ context.Context, e *entity.ConversationEvent) error {
	r.items = append(r.items, e)
	return nil
}
func (r *fakeEventRepo) ListByConversation(_ context.Context, _ string, _ shared.PageRequest) ([]*entity.ConversationEvent, error) {
	return r.items, nil
}

type fakeSectorRepo struct {
	sectorrepo.SectorRepository
	exists map[string]string // id -> tenant
}

func (r *fakeSectorRepo) FindByID(ctx context.Context, id string) (*sectorentity.Sector, error) {
	tenant, _ := shared.TenantFrom(ctx)
	if tid, ok := r.exists[id]; ok && tid == tenant {
		return &sectorentity.Sector{ID: id, TenantID: tenant}, nil
	}
	return nil, apperror.NotFound("not found")
}

type capturedEvent struct{ topic, event string }
type fakePublisher struct{ events []capturedEvent }

func (p *fakePublisher) Publish(_ context.Context, topic, event string, _ any) error {
	p.events = append(p.events, capturedEvent{topic, event})
	return nil
}

// ── fixture ──────────────────────────────────────────────────────────────────

func actorCtx(tenant, userID string, scope authz.SectorScope, sectorIDs []string, perms ...authz.Permission) context.Context {
	ctx := shared.WithTenant(context.Background(), tenant)
	ac := authz.NewAuthContext(tenant, userID, perms, sectorIDs, scope)
	return authz.WithAuthContext(ctx, ac)
}

func newService(sectors map[string]string) (*Service, *fakeConvRepo, *fakeMsgRepo, *fakeEventRepo, *fakePublisher) {
	cr := &fakeConvRepo{items: map[string]*entity.Conversation{}}
	mr := &fakeMsgRepo{}
	er := &fakeEventRepo{}
	pub := &fakePublisher{}
	svc := New(cr, mr, er, &fakeSectorRepo{exists: sectors}, pub, fixedClock{t: time.Unix(1700000000, 0).UTC()})
	return svc, cr, mr, er, pub
}

// adminCtx sees everything (all-sector scope).
func adminCtx() context.Context {
	return actorCtx("t1", "admin", authz.ScopeAll, nil)
}

// ── tests ────────────────────────────────────────────────────────────────────

func TestCreate_DefaultsAndEvent(t *testing.T) {
	svc, _, _, er, pub := newService(map[string]string{"s1": "t1"})
	conv, err := svc.Create(adminCtx(), contracts.CreateConversation{
		ContactID: "c1", Channel: "whatsapp", SectorID: "s1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if conv.Status != entity.StatusNew {
		t.Errorf("status = %q, want new", conv.Status)
	}
	if conv.Priority != entity.PriorityNormal {
		t.Errorf("priority = %q, want normal", conv.Priority)
	}
	if len(er.items) != 1 || er.items[0].Type != entity.EventConversationCreated {
		t.Errorf("expected conversation.created event, got %+v", er.items)
	}
	if len(pub.events) == 0 {
		t.Error("expected a realtime conversation.updated publish")
	}
}

func TestCreate_UnknownSector(t *testing.T) {
	svc, _, _, _, _ := newService(map[string]string{})
	_, err := svc.Create(adminCtx(), contracts.CreateConversation{ContactID: "c1", Channel: "wa", SectorID: "ghost"})
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("expected validation_error, got %v", err)
	}
}

func TestSendMessage_PendingAndEventsAndLastMessageAt(t *testing.T) {
	svc, cr, mr, er, pub := newService(map[string]string{"s1": "t1"})
	ctx := adminCtx()
	conv, _ := svc.Create(ctx, contracts.CreateConversation{ContactID: "c1", Channel: "wa", SectorID: "s1"})

	pub.events = nil
	er.items = nil
	msg, err := svc.SendMessage(ctx, conv.ID, contracts.SendMessage{Text: "hello"})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if msg.Direction != entity.DirectionOutbound {
		t.Errorf("direction = %q, want outbound", msg.Direction)
	}
	if msg.DeliveryStatus != entity.DeliveryPending {
		t.Errorf("delivery_status = %q, want pending", msg.DeliveryStatus)
	}
	if msg.SenderType != entity.SenderAgent {
		t.Errorf("sender_type = %q, want agent", msg.SenderType)
	}
	// last_message_at bumped on the stored conversation.
	if !cr.items[conv.ID].LastMessageAt.Equal(msg.CreatedAt) {
		t.Error("last_message_at not updated")
	}
	// message.created event recorded.
	if len(mr.items) != 1 {
		t.Fatalf("expected 1 message, got %d", len(mr.items))
	}
	foundEvent := false
	for _, e := range er.items {
		if e.Type == entity.EventMessageCreated {
			foundEvent = true
		}
	}
	if !foundEvent {
		t.Error("expected message.created conversation event")
	}
	// realtime message.created + conversation.updated published.
	var gotMsg, gotConv bool
	for _, e := range pub.events {
		switch e.event {
		case contracts.RealtimeMessageCreated:
			gotMsg = true
		case contracts.RealtimeConversationUpdated:
			gotConv = true
		}
	}
	if !gotMsg || !gotConv {
		t.Errorf("expected both realtime events, got %+v", pub.events)
	}
}

func TestInternalNote_IsInternalDirection(t *testing.T) {
	svc, _, _, _, _ := newService(map[string]string{"s1": "t1"})
	ctx := adminCtx()
	conv, _ := svc.Create(ctx, contracts.CreateConversation{ContactID: "c1", Channel: "wa", SectorID: "s1"})

	note, err := svc.AddInternalNote(ctx, conv.ID, contracts.AddInternalNote{Text: "internal"})
	if err != nil {
		t.Fatalf("note: %v", err)
	}
	if note.Direction != entity.DirectionInternal {
		t.Errorf("direction = %q, want internal", note.Direction)
	}
	if note.DeliveryStatus != entity.DeliveryNone {
		t.Errorf("internal note must not be deliverable, got %q", note.DeliveryStatus)
	}
}

func TestCloseAndReopen(t *testing.T) {
	svc, _, _, _, _ := newService(map[string]string{"s1": "t1"})
	ctx := adminCtx()
	conv, _ := svc.Create(ctx, contracts.CreateConversation{ContactID: "c1", Channel: "wa", SectorID: "s1"})

	closed, err := svc.Close(ctx, conv.ID, contracts.CloseConversation{CloseReasonID: "solved", Note: "done"})
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	if closed.Status != entity.StatusClosed || closed.ClosedAt == nil {
		t.Errorf("expected closed with closed_at, got %+v", closed)
	}
	// Sending to a closed conversation is rejected.
	if _, err := svc.SendMessage(ctx, conv.ID, contracts.SendMessage{Text: "x"}); apperror.From(err).Code != apperror.CodeConflict {
		t.Errorf("expected conflict sending to closed, got %v", err)
	}
	// Double close → conflict.
	if _, err := svc.Close(ctx, conv.ID, contracts.CloseConversation{}); apperror.From(err).Code != apperror.CodeConflict {
		t.Errorf("expected conflict on double close, got %v", err)
	}
	// Reopen.
	reopened, err := svc.Reopen(ctx, conv.ID)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if reopened.Status.IsClosed() || reopened.ClosedAt != nil {
		t.Errorf("expected reopened, got %+v", reopened)
	}
	// Reopen again → conflict (not closed).
	if _, err := svc.Reopen(ctx, conv.ID); apperror.From(err).Code != apperror.CodeConflict {
		t.Errorf("expected conflict reopening open conversation, got %v", err)
	}
}

func TestVisibility_AgentSeesOwnSectorOrAssigned(t *testing.T) {
	svc, _, _, _, _ := newService(map[string]string{"s1": "t1", "s2": "t1"})

	// Seed: one conv in s1, one in s2 assigned to bob.
	admin := adminCtx()
	inS1, _ := svc.Create(admin, contracts.CreateConversation{ContactID: "c1", Channel: "wa", SectorID: "s1"})
	inS2Assigned, _ := svc.Create(admin, contracts.CreateConversation{ContactID: "c2", Channel: "wa", SectorID: "s2", AssignedTo: "bob"})
	inS2Other, _ := svc.Create(admin, contracts.CreateConversation{ContactID: "c3", Channel: "wa", SectorID: "s2"})

	// Bob: scope own, member of s1, assigned inS2Assigned.
	bob := actorCtx("t1", "bob", authz.ScopeOwn, []string{"s1"})

	if _, err := svc.Get(bob, inS1.ID); err != nil {
		t.Errorf("bob should see own-sector conversation: %v", err)
	}
	if _, err := svc.Get(bob, inS2Assigned.ID); err != nil {
		t.Errorf("bob should see assigned conversation: %v", err)
	}
	if _, err := svc.Get(bob, inS2Other.ID); apperror.From(err).Code != apperror.CodeNotFound {
		t.Errorf("bob must NOT see other-sector conversation, got %v", err)
	}

	// List is also scoped.
	list, _ := svc.List(bob, contracts.ListFilter{}, shared.PageRequest{Limit: 50})
	if len(list) != 2 {
		t.Errorf("bob should list exactly 2 visible conversations, got %d", len(list))
	}
}

func TestSendMessage_RequiresText(t *testing.T) {
	svc, _, _, _, _ := newService(map[string]string{"s1": "t1"})
	ctx := adminCtx()
	conv, _ := svc.Create(ctx, contracts.CreateConversation{ContactID: "c1", Channel: "wa", SectorID: "s1"})
	if _, err := svc.SendMessage(ctx, conv.ID, contracts.SendMessage{}); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("expected validation_error for empty message, got %v", err)
	}
}

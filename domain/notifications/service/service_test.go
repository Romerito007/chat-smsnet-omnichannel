package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	iamentity "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/notifications/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/notifications/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// ── fakes ────────────────────────────────────────────────────────────────────

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type fakeNotifRepo struct {
	items map[string]*entity.Notification
}

func newNotifRepo() *fakeNotifRepo { return &fakeNotifRepo{items: map[string]*entity.Notification{}} }
func (r *fakeNotifRepo) Create(_ context.Context, n *entity.Notification) error {
	cp := *n
	r.items[n.ID] = &cp
	return nil
}
func (r *fakeNotifRepo) FindByID(_ context.Context, id string) (*entity.Notification, error) {
	if n, ok := r.items[id]; ok {
		cp := *n
		return &cp, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeNotifRepo) ListByUser(_ context.Context, userID string, unreadOnly bool, _ shared.PageRequest) ([]*entity.Notification, error) {
	var out []*entity.Notification
	for _, n := range r.items {
		if n.UserID == userID && (!unreadOnly || !n.Read) {
			out = append(out, n)
		}
	}
	return out, nil
}
func (r *fakeNotifRepo) MarkRead(_ context.Context, id, userID string, at time.Time) error {
	n, ok := r.items[id]
	if !ok || n.UserID != userID {
		return apperror.NotFound("nf")
	}
	n.Read = true
	n.ReadAt = &at
	return nil
}
func (r *fakeNotifRepo) DeleteReadBefore(_ context.Context, before time.Time) (int, error) {
	count := 0
	for id, n := range r.items {
		if n.Read && !n.CreatedAt.After(before) {
			delete(r.items, id)
			count++
		}
	}
	return count, nil
}
func (r *fakeNotifRepo) MarkAllRead(_ context.Context, userID string, at time.Time) (int, error) {
	count := 0
	for _, n := range r.items {
		if n.UserID == userID && !n.Read {
			n.Read = true
			n.ReadAt = &at
			count++
		}
	}
	return count, nil
}

type fakePrefsRepo struct {
	byUser map[string]*entity.Preferences
}

func newPrefsRepo() *fakePrefsRepo { return &fakePrefsRepo{byUser: map[string]*entity.Preferences{}} }
func (r *fakePrefsRepo) FindByUser(_ context.Context, userID string) (*entity.Preferences, error) {
	if p, ok := r.byUser[userID]; ok {
		return p, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakePrefsRepo) Upsert(_ context.Context, p *entity.Preferences) error {
	cp := *p
	r.byUser[p.UserID] = &cp
	return nil
}

type fakeUserRepo struct{ byID map[string]*iamentity.User }

func (r *fakeUserRepo) Create(context.Context, *iamentity.User) error { return nil }
func (r *fakeUserRepo) Update(context.Context, *iamentity.User) error { return nil }
func (r *fakeUserRepo) Delete(context.Context, string) error          { return nil }
func (r *fakeUserRepo) FindByID(_ context.Context, id string) (*iamentity.User, error) {
	if u, ok := r.byID[id]; ok {
		return u, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeUserRepo) FindByIDs(_ context.Context, ids []string) ([]*iamentity.User, error) {
	var out []*iamentity.User
	for _, id := range ids {
		if u, ok := r.byID[id]; ok {
			out = append(out, u)
		}
	}
	return out, nil
}
func (r *fakeUserRepo) FindByEmail(context.Context, string) (*iamentity.User, error) {
	return nil, apperror.NotFound("nf")
}
func (r *fakeUserRepo) FindByEmailAnyTenant(context.Context, string) (*iamentity.User, error) {
	return nil, apperror.NotFound("nf")
}
func (r *fakeUserRepo) List(context.Context, shared.PageRequest) ([]*iamentity.User, error) {
	return nil, nil
}
func (r *fakeUserRepo) ListBySector(context.Context, string) ([]*iamentity.User, error) {
	return nil, nil
}

type captured struct {
	topic, event string
	data         any
}
type fakePublisher struct{ events []captured }

func (p *fakePublisher) Publish(_ context.Context, topic, event string, data any) error {
	p.events = append(p.events, captured{topic, event, data})
	return nil
}

type fakeEmailEnqueuer struct{ tasks []contracts.EmailTask }

func (e *fakeEmailEnqueuer) EnqueueEmail(t contracts.EmailTask) error {
	e.tasks = append(e.tasks, t)
	return nil
}

type fakeEmailSender struct{ sent []contracts.EmailMessage }

func (s *fakeEmailSender) Send(_ context.Context, m contracts.EmailMessage) error {
	s.sent = append(s.sent, m)
	return nil
}

// ── fixture ──────────────────────────────────────────────────────────────────

type fixture struct {
	svc   *Service
	notif *fakeNotifRepo
	prefs *fakePrefsRepo
	pub   *fakePublisher
	enq   *fakeEmailEnqueuer
	send  *fakeEmailSender
}

func newFixture() fixture {
	notif := newNotifRepo()
	prefs := newPrefsRepo()
	users := &fakeUserRepo{byID: map[string]*iamentity.User{
		"u1": {ID: "u1", TenantID: "t1", Email: "agent@example.com"},
	}}
	pub := &fakePublisher{}
	enq := &fakeEmailEnqueuer{}
	send := &fakeEmailSender{}
	svc := NewService(notif, prefs, users, pub, enq, send, fixedClock{t: time.Unix(1700000000, 0).UTC()})
	return fixture{svc: svc, notif: notif, prefs: prefs, pub: pub, enq: enq, send: send}
}

func tenantCtx() context.Context { return shared.WithTenant(context.Background(), "t1") }
func userCtx() context.Context {
	return authz.WithAuthContext(tenantCtx(), authz.NewAuthContext("t1", "u1", nil, nil, authz.ScopeAll))
}

func sendTask(typ string) contracts.SendTask {
	return contracts.SendTask{TenantID: "t1", UserID: "u1", Type: typ, Title: "T", Body: "SECRET-BODY", Link: "/conversations/c1"}
}

// ── tests ────────────────────────────────────────────────────────────────────

func TestSend_CreatesInAppAndRealtime(t *testing.T) {
	fx := newFixture()
	if err := fx.svc.Send(tenantCtx(), sendTask(string(entity.TypeAssignedToYou))); err != nil {
		t.Fatalf("send: %v", err)
	}
	if len(fx.notif.items) != 1 {
		t.Fatalf("expected 1 in-app notification, got %d", len(fx.notif.items))
	}
	// realtime notification.created to the recipient's user topic, carrying the link
	// (the front extracts e.g. the channel id from it).
	found := false
	for _, e := range fx.pub.events {
		if e.event == contracts.RealtimeNotificationCreated && strings.Contains(e.topic, "u1") {
			found = true
			if p, ok := e.data.(realtimePayload); ok {
				if p.Link != "/conversations/c1" {
					t.Errorf("realtime payload must carry the link, got %q", p.Link)
				}
			} else {
				t.Errorf("realtime payload type = %T, want realtimePayload", e.data)
			}
		}
	}
	if !found {
		t.Errorf("expected realtime notification.created to user topic, got %+v", fx.pub.events)
	}
	// assigned_to_you defaults to no email.
	if len(fx.enq.tasks) != 0 {
		t.Errorf("assigned_to_you should not email by default")
	}
}

func TestSend_EmailGatedByDefault(t *testing.T) {
	fx := newFixture()
	// sla.breached defaults to email ON.
	if err := fx.svc.Send(tenantCtx(), sendTask(string(entity.TypeSLABreached))); err != nil {
		t.Fatalf("send: %v", err)
	}
	if len(fx.enq.tasks) != 1 {
		t.Errorf("sla.breached should enqueue email by default, got %d", len(fx.enq.tasks))
	}
}

func TestSend_EmailGatedByPreference(t *testing.T) {
	fx := newFixture()
	// Opt OUT of email for sla.breached.
	fx.prefs.byUser["u1"] = &entity.Preferences{TenantID: "t1", UserID: "u1", EmailByType: map[entity.Type]bool{entity.TypeSLABreached: false}}
	if err := fx.svc.Send(tenantCtx(), sendTask(string(entity.TypeSLABreached))); err != nil {
		t.Fatalf("send: %v", err)
	}
	if len(fx.enq.tasks) != 0 {
		t.Errorf("preference opt-out should suppress email")
	}

	// Opt IN to email for assigned_to_you (default off).
	fx2 := newFixture()
	fx2.prefs.byUser["u1"] = &entity.Preferences{TenantID: "t1", UserID: "u1", EmailByType: map[entity.Type]bool{entity.TypeAssignedToYou: true}}
	if err := fx2.svc.Send(tenantCtx(), sendTask(string(entity.TypeAssignedToYou))); err != nil {
		t.Fatalf("send: %v", err)
	}
	if len(fx2.enq.tasks) != 1 {
		t.Errorf("preference opt-in should enqueue email")
	}
}

func TestSendEmail_NoSensitiveBody(t *testing.T) {
	fx := newFixture()
	// Seed a notification with a sensitive body.
	_ = fx.svc.Send(tenantCtx(), sendTask(string(entity.TypeSLABreached)))
	var id string
	for k := range fx.notif.items {
		id = k
	}
	if err := fx.svc.SendEmail(tenantCtx(), contracts.EmailTask{TenantID: "t1", NotificationID: id}); err != nil {
		t.Fatalf("send email: %v", err)
	}
	if len(fx.send.sent) != 1 {
		t.Fatalf("expected one email sent")
	}
	msg := fx.send.sent[0]
	if msg.To != "agent@example.com" {
		t.Errorf("wrong recipient: %q", msg.To)
	}
	// The sensitive body must NOT appear anywhere in the email.
	blob := msg.Subject + "|" + msg.Link + "|" + msg.Preview
	if strings.Contains(blob, "SECRET-BODY") {
		t.Errorf("email leaked the notification body: %q", blob)
	}
	if msg.Link != "/conversations/c1" {
		t.Errorf("email should carry the link, got %q", msg.Link)
	}
}

func TestMarkReadAndAll(t *testing.T) {
	fx := newFixture()
	_ = fx.svc.Send(tenantCtx(), sendTask(string(entity.TypeAssignedToYou)))
	_ = fx.svc.Send(tenantCtx(), sendTask(string(entity.TypeMention)))
	var first string
	for k := range fx.notif.items {
		first = k
		break
	}
	if err := fx.svc.MarkRead(userCtx(), first); err != nil {
		t.Fatalf("mark read: %v", err)
	}
	if !fx.notif.items[first].Read {
		t.Errorf("notification should be read")
	}
	n, err := fx.svc.MarkAllRead(userCtx())
	if err != nil {
		t.Fatalf("mark all: %v", err)
	}
	if n != 1 { // one was already read
		t.Errorf("expected 1 remaining marked, got %d", n)
	}
}

func TestMarkRead_OtherUsersNotificationDenied(t *testing.T) {
	fx := newFixture()
	// A notification belonging to a different user.
	other := &entity.Notification{ID: "x", TenantID: "t1", UserID: "u2", Type: entity.TypeMention}
	fx.notif.items["x"] = other
	if err := fx.svc.MarkRead(userCtx(), "x"); apperror.From(err).Code != apperror.CodeNotFound {
		t.Errorf("u1 must not mark u2's notification, got %v", err)
	}
}

func TestPreferences_DefaultsAndUpdate(t *testing.T) {
	fx := newFixture()
	prefs, err := fx.svc.Preferences(userCtx())
	if err != nil {
		t.Fatalf("prefs: %v", err)
	}
	eff := prefs.Effective()
	if eff[entity.TypeSLABreached] != true || eff[entity.TypeAssignedToYou] != false {
		t.Errorf("unexpected default effective prefs: %+v", eff)
	}
	// Update: turn assigned_to_you email on.
	updated, err := fx.svc.UpdatePreferences(userCtx(), contracts.UpdatePreferences{
		EmailByType: map[entity.Type]bool{entity.TypeAssignedToYou: true},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !updated.EmailEnabled(entity.TypeAssignedToYou) {
		t.Errorf("expected assigned_to_you email enabled after update")
	}
}

func TestCleanup_DeletesOldReadOnly(t *testing.T) {
	fx := newFixture()
	old := time.Unix(1700000000, 0).UTC()
	// read+old → deleted; read+new → kept; unread+old → kept.
	fx.notif.items["a"] = &entity.Notification{ID: "a", TenantID: "t1", UserID: "u1", Read: true, CreatedAt: old.Add(-48 * time.Hour)}
	fx.notif.items["b"] = &entity.Notification{ID: "b", TenantID: "t1", UserID: "u1", Read: true, CreatedAt: old}
	fx.notif.items["c"] = &entity.Notification{ID: "c", TenantID: "t1", UserID: "u1", Read: false, CreatedAt: old.Add(-48 * time.Hour)}

	n, err := fx.svc.Cleanup(tenantCtx(), old.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 deleted, got %d", n)
	}
	if _, ok := fx.notif.items["a"]; ok {
		t.Errorf("old read notification should be deleted")
	}
	if _, ok := fx.notif.items["b"]; !ok {
		t.Errorf("recent read notification must be kept")
	}
	if _, ok := fx.notif.items["c"]; !ok {
		t.Errorf("unread notification must be kept")
	}
}

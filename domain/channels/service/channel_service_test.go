package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	chcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/contracts"
	chentity "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// ── shared fakes (used across channel/inbound/outbound tests) ─────────────────

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type fakeConnRepo struct {
	byID    map[string]*chentity.ChannelConnection
	byToken map[string]*chentity.ChannelConnection
}

func newFakeConnRepo() *fakeConnRepo {
	return &fakeConnRepo{
		byID:    map[string]*chentity.ChannelConnection{},
		byToken: map[string]*chentity.ChannelConnection{},
	}
}
func (r *fakeConnRepo) put(c *chentity.ChannelConnection) {
	r.byID[c.ID] = c
	r.byToken[c.InboundTokenHash] = c
}
func (r *fakeConnRepo) Create(_ context.Context, c *chentity.ChannelConnection) error {
	r.put(c)
	return nil
}
func (r *fakeConnRepo) Update(_ context.Context, c *chentity.ChannelConnection) error {
	r.put(c)
	return nil
}
func (r *fakeConnRepo) Delete(_ context.Context, id string) error { delete(r.byID, id); return nil }
func (r *fakeConnRepo) FindByID(_ context.Context, id string) (*chentity.ChannelConnection, error) {
	if c, ok := r.byID[id]; ok {
		return c, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeConnRepo) FindByInboundTokenHash(_ context.Context, tokenHash string) (*chentity.ChannelConnection, error) {
	if c, ok := r.byToken[tokenHash]; ok {
		return c, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeConnRepo) List(context.Context, shared.PageRequest) ([]*chentity.ChannelConnection, error) {
	out := make([]*chentity.ChannelConnection, 0, len(r.byID))
	for _, c := range r.byID {
		cp := *c
		out = append(out, &cp)
	}
	return out, nil
}

// fakeHealthChecker reports the connections in `down` as unhealthy.
type fakeHealthChecker struct{ down map[string]bool }

func (f fakeHealthChecker) Check(_ context.Context, conn *chentity.ChannelConnection) error {
	if f.down[conn.ID] {
		return apperror.Integration("down")
	}
	return nil
}

func TestHealthCheck_MarksStatusIdempotently(t *testing.T) {
	svc, repo, _ := newConnService()
	ctx := tenantCtx()
	// a: healthy (currently error → should flip to connected)
	// b: down (currently connected → should flip to error)
	// c: disabled (skipped)
	repo.put(&chentity.ChannelConnection{ID: "a", TenantID: "t1", Type: chentity.TypeWhatsApp, Status: chentity.StatusError, Enabled: true})
	repo.put(&chentity.ChannelConnection{ID: "b", TenantID: "t1", Type: chentity.TypeWebchat, Status: chentity.StatusConnected, Enabled: true})
	repo.put(&chentity.ChannelConnection{ID: "c", TenantID: "t1", Type: chentity.TypeCustom, Status: chentity.StatusConnected, Enabled: false})
	svc.SetHealthChecker(fakeHealthChecker{down: map[string]bool{"b": true}})

	n, err := svc.HealthCheck(ctx)
	if err != nil {
		t.Fatalf("health check: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 status changes, got %d", n)
	}
	if repo.byID["a"].Status != chentity.StatusConnected {
		t.Errorf("a should be connected, got %s", repo.byID["a"].Status)
	}
	if repo.byID["b"].Status != chentity.StatusError {
		t.Errorf("b should be error, got %s", repo.byID["b"].Status)
	}
	if repo.byID["c"].Status != chentity.StatusConnected {
		t.Errorf("disabled c must be left untouched")
	}

	// Idempotent: re-running changes nothing.
	n2, _ := svc.HealthCheck(ctx)
	if n2 != 0 {
		t.Errorf("second run should change 0, got %d", n2)
	}
}

// fakeAdapter is a configurable channel adapter.
type fakeAdapter struct {
	failSend bool
}

func (a *fakeAdapter) Type() chentity.Type { return chentity.TypeCustom }
func (a *fakeAdapter) SendMessage(_ context.Context, _ *chentity.ChannelConnection, send chcontracts.OutboundSend) (chcontracts.SendResult, error) {
	if a.failSend || strings.HasPrefix(send.Text, "FAIL") {
		return chcontracts.SendResult{}, errors.New("send failed")
	}
	return chcontracts.SendResult{ExternalMessageID: "ext-" + shared.NewID(), Status: chentity.DeliverySent}, nil
}
func (a *fakeAdapter) VerifyInbound(conn *chentity.ChannelConnection, _ []byte, headers map[string]string) error {
	if conn.Secret == "" {
		return nil
	}
	if headers["X-Integration-Secret"] == conn.Secret {
		return nil
	}
	return errors.New("verification failed")
}
func (a *fakeAdapter) ParseDeliveryReceipt(_ []byte) ([]chcontracts.DeliveryReceipt, error) {
	return nil, nil
}

type fakeRegistry struct{ adapter *fakeAdapter }

func (r fakeRegistry) For(chentity.Type) chcontracts.Adapter { return r.adapter }

func clockNow() fixedClock { return fixedClock{t: time.Unix(1700000000, 0).UTC()} }

func tenantCtx() context.Context { return shared.WithTenant(context.Background(), "t1") }

// ── tests ────────────────────────────────────────────────────────────────────

func newConnService() (*ConnectionService, *fakeConnRepo, *fakeAdapter) {
	repo := newFakeConnRepo()
	adapter := &fakeAdapter{}
	return NewConnectionService(repo, fakeRegistry{adapter}, clockNow()), repo, adapter
}

func TestCreateConnection_GeneratesTokenAndDisconnected(t *testing.T) {
	svc, _, _ := newConnService()
	conn, err := svc.Create(tenantCtx(), chcontracts.CreateConnection{Type: chentity.TypeWhatsApp, Name: "WA", Secret: "s"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if conn.InboundToken == "" {
		t.Error("expected a plaintext inbound token on creation")
	}
	if conn.InboundTokenHash != hashInboundToken(conn.InboundToken) {
		t.Error("stored hash must match the issued token")
	}
	if conn.Status != chentity.StatusDisconnected {
		t.Errorf("status = %q, want disconnected", conn.Status)
	}
}

func TestCreateConnection_InvalidType(t *testing.T) {
	svc, _, _ := newConnService()
	if _, err := svc.Create(tenantCtx(), chcontracts.CreateConnection{Type: "carrier-pigeon"}); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("expected validation_error, got %v", err)
	}
}

func TestCreateConnection_MessengerTypeIsValid(t *testing.T) {
	svc, repo, _ := newConnService()
	conn, err := svc.Create(tenantCtx(), chcontracts.CreateConnection{Type: chentity.TypeMessenger, Name: "FB"})
	if err != nil {
		t.Fatalf("create messenger connection: %v", err)
	}
	if conn.Type != chentity.TypeMessenger || repo.byID[conn.ID].Type != chentity.TypeMessenger {
		t.Errorf("messenger connection not persisted with the right type: %+v", conn)
	}
}

func validHours() map[string]any {
	return map[string]any{
		"timezone": "America/Sao_Paulo",
		"weekly": []any{
			map[string]any{"day": 1, "intervals": []any{
				map[string]any{"start": "09:00", "end": "12:00"},
				map[string]any{"start": "13:00", "end": "18:00"},
			}},
		},
	}
}

// invalidHours has an interval whose end is not after its start (no overnight).
func invalidHours() map[string]any {
	return map[string]any{
		"timezone": "UTC",
		"weekly":   []any{map[string]any{"day": 1, "intervals": []any{map[string]any{"start": "22:00", "end": "02:00"}}}},
	}
}

func TestCreateConnection_BusinessHoursValidationAndPersist(t *testing.T) {
	svc, repo, _ := newConnService()

	// Invalid business_hours → validation_error, nothing persisted.
	if _, err := svc.Create(tenantCtx(), chcontracts.CreateConnection{
		Type: chentity.TypeWhatsApp, BusinessHours: invalidHours(),
	}); apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("expected validation_error for bad business_hours, got %v", err)
	}

	// Valid business_hours → persisted on the connection.
	conn, err := svc.Create(tenantCtx(), chcontracts.CreateConnection{
		Type: chentity.TypeWhatsApp, BusinessHours: validHours(),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if repo.byID[conn.ID].BusinessHours["timezone"] != "America/Sao_Paulo" {
		t.Errorf("business_hours not persisted, got %v", repo.byID[conn.ID].BusinessHours)
	}
}

func TestUpdateConnection_BusinessHoursValidationAndPersist(t *testing.T) {
	svc, repo, _ := newConnService()
	conn, _ := svc.Create(tenantCtx(), chcontracts.CreateConnection{Type: chentity.TypeWhatsApp})

	// Invalid business_hours on update → validation_error.
	bad := invalidHours()
	if _, err := svc.Update(tenantCtx(), conn.ID, chcontracts.UpdateConnection{BusinessHours: &bad}); apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("expected validation_error on update, got %v", err)
	}

	// Valid business_hours on update → persisted.
	good := validHours()
	if _, err := svc.Update(tenantCtx(), conn.ID, chcontracts.UpdateConnection{BusinessHours: &good}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if repo.byID[conn.ID].BusinessHours["timezone"] != "America/Sao_Paulo" {
		t.Errorf("updated business_hours not persisted, got %v", repo.byID[conn.ID].BusinessHours)
	}
}

func TestTest_SuccessAndFailureUpdateStatus(t *testing.T) {
	svc, _, adapter := newConnService()
	conn, _ := svc.Create(tenantCtx(), chcontracts.CreateConnection{Type: chentity.TypeCustom, Secret: "s"})

	res, updated, err := svc.Test(tenantCtx(), conn.ID)
	if err != nil {
		t.Fatalf("test: %v", err)
	}
	if !res.OK || updated.Status != chentity.StatusConnected {
		t.Errorf("expected OK + connected, got %+v / %s", res, updated.Status)
	}

	adapter.failSend = true
	res, updated, _ = svc.Test(tenantCtx(), conn.ID)
	if res.OK || updated.Status != chentity.StatusError {
		t.Errorf("expected failure + error status, got %+v / %s", res, updated.Status)
	}
}

func TestResolveInbound(t *testing.T) {
	svc, repo, _ := newConnService()
	conn := &chentity.ChannelConnection{
		ID: "c1", TenantID: "t1", Type: chentity.TypeWhatsApp, Enabled: true,
		InboundTokenHash: hashInboundToken("tok"), Secret: "s3cr3t",
	}
	repo.put(conn)

	okHeaders := map[string]string{"X-Integration-Secret": "s3cr3t"}

	// happy path
	got, err := svc.ResolveInbound(context.Background(), "tok", chentity.TypeWhatsApp, []byte("{}"), okHeaders)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got.TenantID != "t1" {
		t.Errorf("tenant = %q", got.TenantID)
	}

	// unknown token
	if _, err := svc.ResolveInbound(context.Background(), "ghost", chentity.TypeWhatsApp, []byte("{}"), okHeaders); apperror.From(err).Code != apperror.CodeUnauthorized {
		t.Errorf("unknown token: want unauthorized, got %v", err)
	}
	// channel mismatch
	if _, err := svc.ResolveInbound(context.Background(), "tok", chentity.TypeTelegram, []byte("{}"), okHeaders); apperror.From(err).Code != apperror.CodeUnauthorized {
		t.Errorf("channel mismatch: want unauthorized, got %v", err)
	}
	// bad secret
	if _, err := svc.ResolveInbound(context.Background(), "tok", chentity.TypeWhatsApp, []byte("{}"), map[string]string{"X-Integration-Secret": "nope"}); apperror.From(err).Code != apperror.CodeUnauthorized {
		t.Errorf("bad secret: want unauthorized, got %v", err)
	}
	// disabled
	conn.Enabled = false
	repo.put(conn)
	if _, err := svc.ResolveInbound(context.Background(), "tok", chentity.TypeWhatsApp, []byte("{}"), okHeaders); apperror.From(err).Code != apperror.CodeUnauthorized {
		t.Errorf("disabled: want unauthorized, got %v", err)
	}
}

// recordingWebhookManager records the last managed-webhook sync, to assert that a
// secret rotation re-syncs the channel's managed subscription with the new secret.
type recordingWebhookManager struct {
	syncedChannelID string
	syncedURL       string
	syncedSecret    string
}

func (m *recordingWebhookManager) SyncChannelWebhook(_ context.Context, channelID, url, secret string) error {
	m.syncedChannelID, m.syncedURL, m.syncedSecret = channelID, url, secret
	return nil
}
func (m *recordingWebhookManager) RemoveChannelWebhook(context.Context, string) error { return nil }

func TestRotateOutboundSecret_NewSecretRevealedAndManagedWebhookResynced(t *testing.T) {
	svc, _, _ := newConnService()
	wm := &recordingWebhookManager{}
	svc.SetWebhookManager(wm)

	conn, err := svc.Create(tenantCtx(), chcontracts.CreateConnection{
		Type: chentity.TypeAPI, Name: "api", BaseURL: "https://x.example/in", Secret: "old-secret",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	rotated, err := svc.RotateOutboundSecret(tenantCtx(), conn.ID)
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if rotated.Secret == "" || rotated.Secret == "old-secret" {
		t.Fatalf("rotation must produce a new non-empty secret, got %q", rotated.Secret)
	}
	// The managed webhook was re-synced with the channel URL and the NEW secret.
	if wm.syncedChannelID != conn.ID || wm.syncedURL != "https://x.example/in" || wm.syncedSecret != rotated.Secret {
		t.Errorf("managed webhook not re-synced with the new secret: %+v", wm)
	}
}

// fakeNotifier records in-app notifications.
type fakeNotifier struct{ sent []shared.NotifyInput }

func (f *fakeNotifier) Notify(_ context.Context, in shared.NotifyInput) { f.sent = append(f.sent, in) }

// fakeAudience returns a fixed recipient set.
type fakeAudience struct {
	ids []string
	err error
}

func (a fakeAudience) NotifyRecipients(context.Context) ([]string, error) { return a.ids, a.err }

func TestReplaceTemplates_ReplacesPersistsAndNotifies(t *testing.T) {
	svc, repo, _ := newConnService()
	notifier := &fakeNotifier{}
	svc.SetNotifier(notifier)
	svc.SetTemplateAudience(fakeAudience{ids: []string{"u1", "u2"}})
	ctx := tenantCtx()

	repo.put(&chentity.ChannelConnection{
		ID: "c1", TenantID: "t1", Type: chentity.TypeWhatsApp, Name: "WA Vendas",
		WhatsAppTemplates: []chentity.WhatsAppTemplate{{ID: "old", Name: "Antigo"}},
	})

	newTpls := []chentity.WhatsAppTemplate{
		{ID: "t1", Name: "Boas-vindas", Body: chentity.WhatsAppTemplateBody{Text: "Olá {{nome}}",
			Variables: []chentity.WhatsAppTemplateVariable{{Key: "nome"}}}},
	}
	conn, err := svc.ReplaceTemplates(ctx, "c1", newTpls)
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if len(conn.WhatsAppTemplates) != 1 || conn.WhatsAppTemplates[0].ID != "t1" {
		t.Fatalf("templates must be replaced wholesale: %+v", conn.WhatsAppTemplates)
	}
	stored, _ := repo.FindByID(ctx, "c1")
	if len(stored.WhatsAppTemplates) != 1 || stored.WhatsAppTemplates[0].ID != "t1" {
		t.Errorf("replacement must be persisted, got %+v", stored.WhatsAppTemplates)
	}
	// One in-app notification per recipient, of the templates-updated type.
	if len(notifier.sent) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(notifier.sent))
	}
	for _, n := range notifier.sent {
		if n.Type != "channel.templates_updated" {
			t.Errorf("notification type = %q, want channel.templates_updated", n.Type)
		}
		if n.TenantID != "t1" {
			t.Errorf("notification tenant = %q", n.TenantID)
		}
		// The link carries the channel id so the front knows WHICH channel updated.
		if n.Link != "/channels/c1" {
			t.Errorf("notification link = %q, want /channels/c1", n.Link)
		}
	}
}

func TestReplaceTemplates_RejectsInvalidWithoutPersistOrNotify(t *testing.T) {
	svc, repo, _ := newConnService()
	notifier := &fakeNotifier{}
	svc.SetNotifier(notifier)
	svc.SetTemplateAudience(fakeAudience{ids: []string{"u1"}})
	ctx := tenantCtx()
	repo.put(&chentity.ChannelConnection{
		ID: "c1", TenantID: "t1", Type: chentity.TypeWhatsApp,
		WhatsAppTemplates: []chentity.WhatsAppTemplate{{ID: "keep"}},
	})

	// A template with no id is invalid (validateTemplates).
	_, err := svc.ReplaceTemplates(ctx, "c1", []chentity.WhatsAppTemplate{{Name: "no id"}})
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("invalid payload must be a validation error, got %v", err)
	}
	stored, _ := repo.FindByID(ctx, "c1")
	if len(stored.WhatsAppTemplates) != 1 || stored.WhatsAppTemplates[0].ID != "keep" {
		t.Errorf("an invalid push must not overwrite the existing mirror")
	}
	if len(notifier.sent) != 0 {
		t.Errorf("an invalid push must not notify, got %d", len(notifier.sent))
	}
}

// fakeChannelEmitter records EmitToChannel calls (the managed-webhook emit).
type fakeChannelEmitter struct {
	calls   int
	event   string
	channel string
	err     error
}

func (e *fakeChannelEmitter) EmitToChannel(_ context.Context, _, channelID, event string, _ any) error {
	e.calls++
	e.event = event
	e.channel = channelID
	return e.err
}

func TestSyncTemplates_EmitsEventToManagedWebhook(t *testing.T) {
	svc, _, _ := newConnService()
	em := &fakeChannelEmitter{}
	svc.SetChannelEmitter(em)
	if err := svc.SyncTemplates(tenantCtx(), "c1"); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if em.calls != 1 || em.event != "templates_sync_requested" || em.channel != "c1" {
		t.Fatalf("expected one templates_sync_requested emit to c1, got %+v", em)
	}
}

func TestSyncTemplates_NoEmitterConfigured(t *testing.T) {
	svc, _, _ := newConnService()
	if err := svc.SyncTemplates(tenantCtx(), "c1"); apperror.From(err).Code != apperror.CodeIntegrationUnavailable {
		t.Fatalf("no emitter must be an integration error, got %v", err)
	}
}

func TestSyncTemplates_PropagatesEmitterError(t *testing.T) {
	svc, _, _ := newConnService()
	svc.SetChannelEmitter(&fakeChannelEmitter{err: apperror.Conflict("no managed webhook")})
	if err := svc.SyncTemplates(tenantCtx(), "c1"); apperror.From(err).Code != apperror.CodeConflict {
		t.Fatalf("emitter error must propagate, got %v", err)
	}
}

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

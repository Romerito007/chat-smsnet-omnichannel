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
	byType  map[chentity.Type]*chentity.ChannelConnection
}

func newFakeConnRepo() *fakeConnRepo {
	return &fakeConnRepo{
		byID:    map[string]*chentity.ChannelConnection{},
		byToken: map[string]*chentity.ChannelConnection{},
		byType:  map[chentity.Type]*chentity.ChannelConnection{},
	}
}
func (r *fakeConnRepo) put(c *chentity.ChannelConnection) {
	r.byID[c.ID] = c
	r.byToken[c.WebhookVerifyToken] = c
	if c.Enabled {
		r.byType[c.Type] = c
	}
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
func (r *fakeConnRepo) FindEnabledByType(_ context.Context, t chentity.Type) (*chentity.ChannelConnection, error) {
	if c, ok := r.byType[t]; ok {
		return c, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeConnRepo) FindByWebhookVerifyToken(_ context.Context, token string) (*chentity.ChannelConnection, error) {
	if c, ok := r.byToken[token]; ok {
		return c, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeConnRepo) List(context.Context, shared.PageRequest) ([]*chentity.ChannelConnection, error) {
	return nil, nil
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
	if conn.WebhookVerifyToken == "" {
		t.Error("expected a webhook verify token")
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
		WebhookVerifyToken: "tok", Secret: "s3cr3t",
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

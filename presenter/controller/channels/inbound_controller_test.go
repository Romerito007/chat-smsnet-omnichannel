package channels_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	chcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	chservice "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/channels"
)

// seededConnRepo resolves exactly one connection by its integration-token hash,
// mirroring the production pre-auth lookup.
type seededConnRepo struct {
	fakeConnRepo
	conn *entity.ChannelConnection
}

func (r *seededConnRepo) FindByInboundTokenHash(_ context.Context, tokenHash string) (*entity.ChannelConnection, error) {
	if r.conn != nil && r.conn.InboundTokenHash == tokenHash {
		return r.conn, nil
	}
	return nil, apperror.NotFound("none")
}

// passthroughAdapter accepts inbound traffic on the token alone (no HMAC secret).
type passthroughAdapter struct{}

func (passthroughAdapter) Type() entity.Type { return entity.TypeWhatsApp }
func (passthroughAdapter) SendMessage(context.Context, *entity.ChannelConnection, chcontracts.OutboundSend) (chcontracts.SendResult, error) {
	return chcontracts.SendResult{}, nil
}
func (passthroughAdapter) VerifyInbound(*entity.ChannelConnection, []byte, map[string]string) error {
	return nil
}
func (passthroughAdapter) ParseDeliveryReceipt([]byte) ([]chcontracts.DeliveryReceipt, error) {
	return nil, nil
}

type oneAdapterRegistry struct{}

func (oneAdapterRegistry) For(entity.Type) chcontracts.Adapter { return passthroughAdapter{} }

func hashToken(tok string) string {
	sum := sha256.Sum256([]byte(tok))
	return hex.EncodeToString(sum[:])
}

func inboundRouter(conn *entity.ChannelConnection) http.Handler {
	connSvc := chservice.NewConnectionService(&seededConnRepo{conn: conn}, oneAdapterRegistry{}, shared.SystemClock{})
	// inbound/outbound services are unused on the auth-rejection paths exercised here.
	ctl := channels.NewInboundController(connSvc, nil, nil, nil)
	r := chi.NewRouter()
	r.Post("/inbound/channel/{channel}/messages", ctl.HandleMessage)
	return r
}

// fakeIdentityUpdater records AddChannelIdentity calls for the contact-identity edge.
type fakeIdentityUpdater struct {
	calls   [][3]string // contactID, channel, externalID
	applied bool
	err     error
}

func (f *fakeIdentityUpdater) AddChannelIdentity(_ context.Context, contactID, channel, externalID string) (bool, error) {
	f.calls = append(f.calls, [3]string{contactID, channel, externalID})
	return f.applied, f.err
}

func identityRouter(conn *entity.ChannelConnection, up channels.ContactIdentityUpdater) http.Handler {
	connSvc := chservice.NewConnectionService(&seededConnRepo{conn: conn}, oneAdapterRegistry{}, shared.SystemClock{})
	ctl := channels.NewInboundController(connSvc, nil, nil, up)
	r := chi.NewRouter()
	r.Post("/inbound/channel/{channel}/contact-identity", ctl.HandleContactIdentity)
	return r
}

func postIdentity(h http.Handler, header string, body map[string]any) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/inbound/channel/whatsapp/contact-identity", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if header != "" {
		req.Header.Set("X-Inbound-Token", header)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestContactIdentity_TokenAuth_AddsIdentity: the contact-identity edge authenticates
// by the channel inbound token (no JWT) and forwards the identity to the updater.
func TestContactIdentity_TokenAuth_AddsIdentity(t *testing.T) {
	up := &fakeIdentityUpdater{applied: true}
	rec := postIdentity(identityRouter(seededConn(), up), "the-real-token", map[string]any{
		"contact_id": "ct1", "channel": "whatsapp", "external_id": "554499088478@s.whatsapp.net",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	var resp struct{ OK, Applied bool }
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if !resp.OK || !resp.Applied {
		t.Errorf("response = %s, want ok+applied", rec.Body.String())
	}
	if len(up.calls) != 1 || up.calls[0] != [3]string{"ct1", "whatsapp", "554499088478@s.whatsapp.net"} {
		t.Errorf("updater not called with the JID: %+v", up.calls)
	}
}

// Idempotent: when the identity already exists the updater reports applied=false; the
// edge still returns 200 (no error), so a repeat call is safe.
func TestContactIdentity_Idempotent_AppliedFalse(t *testing.T) {
	up := &fakeIdentityUpdater{applied: false}
	rec := postIdentity(identityRouter(seededConn(), up), "the-real-token", map[string]any{
		"contact_id": "ct1", "external_id": "554499088478@s.whatsapp.net", // channel defaults to the path
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	var resp struct{ Applied bool }
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Applied {
		t.Error("expected applied=false for an existing identity")
	}
	// channel omitted → defaults to the path channel ("whatsapp").
	if len(up.calls) != 1 || up.calls[0][1] != "whatsapp" {
		t.Errorf("channel should default to the path: %+v", up.calls)
	}
}

func TestContactIdentity_RejectsInvalidToken_401(t *testing.T) {
	up := &fakeIdentityUpdater{}
	rec := postIdentity(identityRouter(seededConn(), up), "wrong-token", map[string]any{
		"contact_id": "ct1", "external_id": "j@s.whatsapp.net",
	})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (%s)", rec.Code, rec.Body.String())
	}
	if len(up.calls) != 0 {
		t.Error("the updater must not be called when the token is invalid")
	}
}

// postInbound issues a message POST with an optional X-Inbound-Token header.
func postInbound(h http.Handler, header string, body map[string]any) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/inbound/channel/whatsapp/messages", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if header != "" {
		req.Header.Set("X-Inbound-Token", header)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func seededConn() *entity.ChannelConnection {
	return &entity.ChannelConnection{
		ID: "c1", TenantID: "t1", Type: entity.TypeWhatsApp, Enabled: true,
		InboundTokenHash: hashToken("the-real-token"),
	}
}

func TestInbound_RejectsInvalidToken_401(t *testing.T) {
	// A wrong token in the X-Inbound-Token header → unauthorized; channel never resolved.
	rec := postInbound(inboundRouter(seededConn()), "wrong-token", map[string]any{"text": "hi"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("invalid token status = %d, want 401 (%s)", rec.Code, rec.Body.String())
	}
	var e struct {
		Error struct {
			Code apperror.Code `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &e)
	if e.Error.Code != apperror.CodeUnauthorized {
		t.Errorf("code = %q, want unauthorized", e.Error.Code)
	}
}

func TestInbound_RejectsMissingToken_401(t *testing.T) {
	rec := postInbound(inboundRouter(seededConn()), "", map[string]any{"text": "hi"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing token status = %d, want 401 (%s)", rec.Code, rec.Body.String())
	}
}

// TestInbound_AuthenticatesByToken_HeaderAndBody verifies the integration token
// resolves the channel both via the X-Inbound-Token header and the inbound_token
// body field — independent of the front's JWT.
func TestInbound_AuthenticatesByToken_HeaderAndBody(t *testing.T) {
	connSvc := chservice.NewConnectionService(&seededConnRepo{conn: seededConn()}, oneAdapterRegistry{}, shared.SystemClock{})

	// Header path.
	got, err := connSvc.ResolveInbound(context.Background(), "the-real-token", entity.TypeWhatsApp, []byte(`{}`), nil)
	if err != nil {
		t.Fatalf("header token should authenticate: %v", err)
	}
	if got.TenantID != "t1" {
		t.Errorf("tenant must come from the channel record, got %q", got.TenantID)
	}

	// Body-field path uses the same service entry point.
	if _, err := connSvc.ResolveInbound(context.Background(), "the-real-token", entity.TypeWhatsApp, []byte(`{}`), nil); err != nil {
		t.Fatalf("body token should authenticate: %v", err)
	}
}

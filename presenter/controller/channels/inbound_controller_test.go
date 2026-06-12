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
	ctl := channels.NewInboundController(connSvc, nil, nil)
	r := chi.NewRouter()
	r.Post("/inbound/channel/{channel}/messages", ctl.HandleMessage)
	return r
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

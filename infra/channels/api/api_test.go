package api

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	chcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/contracts"
	chentity "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/channels/sign"
)

const testSecret = "outbound-secret-xyz"

// externalSystem simulates the company's external endpoint: it verifies the HMAC
// signature over "Timestamp.body" and records the received envelope/headers.
type externalSystem struct {
	gotEvent     string
	gotDelivery  string
	sigVerified  bool
	body         envelope
	status       int
	responseBody string
}

func newExternalServer(t *testing.T, ext *externalSystem) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		ext.gotEvent = r.Header.Get("X-Chat-Event")
		ext.gotDelivery = r.Header.Get("Delivery-Id")
		ts := r.Header.Get("Timestamp")
		want := sign.Payload(testSecret, ts, body)
		if subtle.ConstantTimeCompare([]byte(want), []byte(r.Header.Get("Signature"))) == 1 {
			ext.sigVerified = true
		}
		_ = json.Unmarshal(body, &ext.body)
		if ext.status != 0 {
			w.WriteHeader(ext.status)
		}
		if ext.responseBody != "" {
			_, _ = w.Write([]byte(ext.responseBody))
		}
	}))
}

func connFor(url string) *chentity.ChannelConnection {
	return &chentity.ChannelConnection{
		ID: "conn-api", TenantID: "t1", Type: chentity.TypeAPI,
		BaseURL: url, Secret: testSecret, Enabled: true,
	}
}

func sampleSend() chcontracts.OutboundSend {
	return chcontracts.OutboundSend{
		DeliveryID:        "del-1",
		ConversationID:    "conv-1",
		ExternalContactID: "ext-c",
		Contact:           chcontracts.OutboundContact{ID: "c1", Name: "Alice", Phone: "5511", ExternalID: "ext-c"},
		Text:              "olá",
		Attachments:       []conventity.Attachment{{URL: "https://f/x.png", ContentType: "image/png", Filename: "x.png", Size: 12}},
	}
}

func TestSendMessage_SignedPOST_Success(t *testing.T) {
	ext := &externalSystem{responseBody: `{"external_message_id":"EXT-99"}`}
	srv := newExternalServer(t, ext)
	defer srv.Close()

	res, err := New(chentity.TypeAPI).SendMessage(t.Context(), connFor(srv.URL), sampleSend())
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if !ext.sigVerified {
		t.Error("external system did not verify the HMAC signature")
	}
	if ext.gotEvent != eventMessage || ext.gotDelivery != "del-1" {
		t.Errorf("headers not set: event=%q delivery=%q", ext.gotEvent, ext.gotDelivery)
	}
	if ext.body.DeliveryID != "del-1" || ext.body.ConversationID != "conv-1" {
		t.Errorf("envelope ids not sent: %+v", ext.body)
	}
	if ext.body.Contact.ExternalID != "ext-c" || ext.body.Message.Text != "olá" {
		t.Errorf("envelope contact/message wrong: %+v", ext.body)
	}
	if len(ext.body.Message.Attachments) != 1 || ext.body.Message.Attachments[0].URL != "https://f/x.png" {
		t.Errorf("attachments not sent: %+v", ext.body.Message.Attachments)
	}
	if res.Status != chentity.DeliverySent || res.ExternalMessageID != "EXT-99" {
		t.Errorf("unexpected result: %+v", res)
	}
}

func TestSendMessage_NoExternalID_FallsBackToDeliveryID(t *testing.T) {
	ext := &externalSystem{responseBody: `{"ok":true}`}
	srv := newExternalServer(t, ext)
	defer srv.Close()

	res, err := New(chentity.TypeAPI).SendMessage(t.Context(), connFor(srv.URL), sampleSend())
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if res.ExternalMessageID != "del-1" {
		t.Errorf("external id should fall back to delivery id, got %q", res.ExternalMessageID)
	}
}

func TestSendMessage_Non2xx_ReturnsErrorForRetry(t *testing.T) {
	ext := &externalSystem{status: http.StatusInternalServerError, responseBody: "boom"}
	srv := newExternalServer(t, ext)
	defer srv.Close()

	if _, err := New(chentity.TypeAPI).SendMessage(t.Context(), connFor(srv.URL), sampleSend()); err == nil {
		t.Fatal("expected an error so the delivery is retried")
	}
}

func TestSendMessage_NoOutboundURL(t *testing.T) {
	conn := connFor("")
	if _, err := New(chentity.TypeAPI).SendMessage(t.Context(), conn, sampleSend()); err == nil {
		t.Fatal("expected an error when outbound_url is not configured")
	}
}

func TestVerifyInbound(t *testing.T) {
	a := New(chentity.TypeAPI)
	conn := &chentity.ChannelConnection{Secret: testSecret}
	body := []byte(`{"external_message_id":"m1","text":"hi"}`)

	// HMAC of the raw body.
	if err := a.VerifyInbound(conn, body, map[string]string{"X-Signature": sign.Body(testSecret, body)}); err != nil {
		t.Errorf("body signature should verify: %v", err)
	}
	// HMAC of "timestamp.body".
	ts := "1700000000000"
	hdr := map[string]string{"Timestamp": ts, "X-Signature": sign.Payload(testSecret, ts, body)}
	if err := a.VerifyInbound(conn, body, hdr); err != nil {
		t.Errorf("timestamped signature should verify: %v", err)
	}
	// Wrong signature.
	if err := a.VerifyInbound(conn, body, map[string]string{"X-Signature": "deadbeef"}); err == nil {
		t.Error("invalid signature must fail")
	}
	// Exact shared secret.
	if err := a.VerifyInbound(conn, body, map[string]string{"X-Integration-Secret": testSecret}); err != nil {
		t.Errorf("shared secret should verify: %v", err)
	}
	// Missing everything.
	if err := a.VerifyInbound(conn, body, map[string]string{}); err == nil {
		t.Error("missing signature must fail")
	}
}

func TestParseDeliveryReceipt(t *testing.T) {
	a := New(chentity.TypeAPI)
	recs, err := a.ParseDeliveryReceipt([]byte(`{"receipts":[
		{"external_message_id":"m1","status":"delivered"},
		{"external_message_id":"m2","status":"read"},
		{"external_message_id":"m3","status":"failed","error":"unreachable"}
	]}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(recs) != 3 {
		t.Fatalf("want 3 receipts, got %d", len(recs))
	}
	if recs[0].Status != chentity.DeliveryDelivered || recs[1].Status != chentity.DeliveryRead {
		t.Errorf("statuses wrong: %+v", recs)
	}
	if recs[2].Status != chentity.DeliveryFailed || recs[2].Error != "unreachable" {
		t.Errorf("failed receipt wrong: %+v", recs[2])
	}
}

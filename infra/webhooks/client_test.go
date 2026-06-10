package webhooks

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/entity"
)

func TestSign_MatchesIndependentHMAC(t *testing.T) {
	secret := "whsec_test"
	ts := "1700000000"
	body := []byte(`{"event":"message.created"}`)

	got := Sign(secret, ts, body)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "."))
	mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))

	if got != want {
		t.Fatalf("signature mismatch:\n got %s\nwant %s", got, want)
	}
}

func TestSend_SetsHeadersAndVerifiableSignature(t *testing.T) {
	sub := &entity.WebhookSubscription{ID: "wh1", Secret: "whsec_test", Events: []string{"message.created"}}
	delivery := &entity.WebhookDelivery{ID: "d1", Event: "message.created", Payload: []byte(`{"hello":"world"}`)}

	var (
		gotEvent, gotDelivery, gotSig, gotTs string
		gotBody                              []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEvent = r.Header.Get(HeaderEvent)
		gotDelivery = r.Header.Get(HeaderDeliveryID)
		gotSig = r.Header.Get(HeaderSignature)
		gotTs = r.Header.Get(HeaderTimestamp)
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	sub.URL = srv.URL

	res, err := NewSender().Send(context.Background(), sub, delivery)
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	if gotEvent != "message.created" || gotDelivery != "d1" {
		t.Errorf("missing event/delivery headers: event=%q delivery=%q", gotEvent, gotDelivery)
	}
	if !strings.HasPrefix(gotSig, "sha256=") {
		t.Fatalf("signature header not prefixed: %q", gotSig)
	}
	// The receiver can verify the signature over the exact received body.
	want := "sha256=" + Sign(sub.Secret, gotTs, gotBody)
	if gotSig != want {
		t.Errorf("signature not verifiable by receiver:\n got %s\nwant %s", gotSig, want)
	}
}

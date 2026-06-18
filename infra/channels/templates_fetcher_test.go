package channels

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	chentity "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
)

// The fetcher GETs {origin}/channels/{type}/templates, signs the EMPTY body exactly
// like an outbound delivery (Signature = HMAC-SHA256(secret, "<ts>." )), and parses
// the {templates:[...]} response into the entity mirror.
func TestTemplateFetcher_SignsEmptyBodyGetAndParses(t *testing.T) {
	const secret = "out-secret"
	var gotPath, gotTS, gotSig, gotMethod string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotTS = r.Header.Get("Timestamp")
		gotSig = r.Header.Get("Signature")
		_, _ = w.Write([]byte(`{"templates":[
			{"id":"t1","name":"Boas-vindas","language":"pt_BR",
			 "body":{"text":"Olá {{nome}}","variables":[{"key":"nome","label":"Nome"}]},
			 "header":{"type":"text","text":"Oi"},
			 "buttons":[{"type":"url","text":"Site","url":"https://x"}],"footer":"tchau"}
		]}`))
	}))
	defer srv.Close()

	conn := &chentity.ChannelConnection{
		ID: "c1", TenantID: "t1", Type: chentity.TypeWhatsApp, BaseURL: srv.URL + "/webhook", Secret: secret,
	}
	got, err := NewTemplateFetcher().FetchTemplates(t.Context(), conn)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}

	if gotMethod != http.MethodGet {
		t.Errorf("must be a GET, got %s", gotMethod)
	}
	if gotPath != "/channels/whatsapp/templates" {
		t.Errorf("path = %q, want /channels/whatsapp/templates", gotPath)
	}
	// The signed material is "<timestamp>." over an EMPTY body — assert byte-for-byte
	// so the gateway's verifier matches.
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(gotTS + "."))
	want := hex.EncodeToString(mac.Sum(nil))
	if gotSig != want {
		t.Errorf("Signature mismatch:\n got  %q\n want %q (HMAC of %q)", gotSig, want, gotTS+".")
	}

	if len(got) != 1 {
		t.Fatalf("want 1 template, got %d", len(got))
	}
	tpl := got[0]
	if tpl.ID != "t1" || tpl.Name != "Boas-vindas" || tpl.Language != "pt_BR" {
		t.Errorf("template fields not mapped: %+v", tpl)
	}
	if tpl.Body.Text != "Olá {{nome}}" || len(tpl.Body.Variables) != 1 || tpl.Body.Variables[0].Key != "nome" {
		t.Errorf("body not mapped: %+v", tpl.Body)
	}
	if tpl.Header == nil || tpl.Header.Text != "Oi" {
		t.Errorf("header not mapped: %+v", tpl.Header)
	}
	if len(tpl.Buttons) != 1 || tpl.Buttons[0].URL != "https://x" {
		t.Errorf("buttons not mapped: %+v", tpl.Buttons)
	}
}

// A non-2xx from the gateway is an error (so the caller keeps the existing mirror).
func TestTemplateFetcher_Non2xxIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("upstream down"))
	}))
	defer srv.Close()

	conn := &chentity.ChannelConnection{ID: "c1", Type: chentity.TypeWhatsApp, BaseURL: srv.URL, Secret: "s"}
	if _, err := NewTemplateFetcher().FetchTemplates(t.Context(), conn); err == nil {
		t.Fatal("a 502 from the gateway must return an error")
	}
}

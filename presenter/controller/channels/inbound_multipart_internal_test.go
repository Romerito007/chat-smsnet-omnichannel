package channels

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

// parseMultipartInbound must carry the group context (group_jid + sender_*) and the
// rich contact/location payloads (as JSON-encoded fields) — so a group contact/
// location sent over multipart is not silently dropped to a 1:1 text message.
func TestParseMultipartInbound_GroupAndRichFields(t *testing.T) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fields := map[string]string{
		"external_message_id": "wamid.m1",
		"group_jid":           "120363025246125486@g.us",
		"sender_jid":          "5544999990000@s.whatsapp.net",
		"sender_name":         "João",
		"sender_phone":        "5544999990000",
		"content":             "compartilhou um contato",
		"contacts":            `[{"name":{"formatted":"Maria"},"phones":[{"phone":"5544111"}]}]`,
		"location":            `{"latitude":-23.55,"longitude":-46.63,"name":"SP"}`,
	}
	for k, v := range fields {
		_ = w.WriteField(k, v)
	}
	_ = w.Close()

	req := httptest.NewRequest(http.MethodPost, "/inbound/channel/whatsapp/messages", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())

	got, _, err := parseMultipartInbound(req)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.GroupJID != "120363025246125486@g.us" {
		t.Errorf("group_jid not parsed: %q", got.GroupJID)
	}
	if got.SenderJID != "5544999990000@s.whatsapp.net" || got.SenderName != "João" {
		t.Errorf("sender not parsed: jid=%q name=%q", got.SenderJID, got.SenderName)
	}
	if len(got.Contacts) != 1 || got.Contacts[0].Name.Formatted != "Maria" {
		t.Errorf("contacts JSON field not parsed: %+v", got.Contacts)
	}
	if got.Location == nil || got.Location.Name != "SP" {
		t.Errorf("location JSON field not parsed: %+v", got.Location)
	}
}

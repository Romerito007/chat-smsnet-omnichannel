package channels

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"net/textproto"
	"testing"
)

// buildMultipart assembles a Chatwoot-style multipart inbound body: the text/routing
// fields plus an optional file part under the given field name, with an explicit
// per-part Content-Type (as the WhatsApp gateway sends).
func buildMultipart(t *testing.T, fields map[string]string, fileField, filename, partCT string, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("write field: %v", err)
		}
	}
	if fileField != "" {
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", `form-data; name="`+fileField+`"; filename="`+filename+`"`)
		h.Set("Content-Type", partCT)
		part, err := w.CreatePart(h)
		if err != nil {
			t.Fatalf("create part: %v", err)
		}
		if _, err := part.Write(data); err != nil {
			t.Fatalf("write part: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	return &buf, w.FormDataContentType()
}

// TestParseMultipart_ImageNoCaption is the inbound-media regression at the parse
// layer: a WhatsApp image under attachments[] with content="" (no caption) must be
// parsed into a single RawFile (keeping its filename + content type) and an empty
// Text — media-only is valid.
func TestParseMultipart_ImageNoCaption(t *testing.T) {
	body, ct := buildMultipart(t,
		map[string]string{
			"external_message_id": "3EB0C97998E561CEA762ED",
			"external_contact_id": "5511999998888",
			"contact_phone":       "+5511999998888",
			"content":             "", // media-only, no caption
			"file_type":           "image",
		},
		"attachments[]", "3EB0C97998E561CEA762ED.jpg", "image/jpeg", []byte("\xff\xd8\xff\xe0jpeg"))

	req := httptest.NewRequest("POST", "/inbound/channel/api/messages", body)
	req.Header.Set("Content-Type", ct)

	parsed, raw, err := parseMultipartInbound(req)
	if err != nil {
		t.Fatalf("parse must accept attachments[] with no caption: %v", err)
	}
	if parsed.Text != "" {
		t.Errorf("media-only message must have empty text, got %q", parsed.Text)
	}
	if parsed.ExternalMessageID != "3EB0C97998E561CEA762ED" || parsed.ContactPhone != "+5511999998888" {
		t.Errorf("routing fields not parsed: %+v", parsed)
	}
	if len(raw) != 1 {
		t.Fatalf("expected exactly 1 raw attachment, got %d", len(raw))
	}
	if raw[0].Filename != "3EB0C97998E561CEA762ED.jpg" || raw[0].ContentType != "image/jpeg" {
		t.Errorf("attachment metadata wrong: name=%q ct=%q", raw[0].Filename, raw[0].ContentType)
	}
	if string(raw[0].Data) != "\xff\xd8\xff\xe0jpeg" {
		t.Errorf("attachment bytes not preserved: %q", raw[0].Data)
	}
}

// TestParseMultipart_ImageWithCaption: content="foto" fills Text alongside the file.
func TestParseMultipart_ImageWithCaption(t *testing.T) {
	body, ct := buildMultipart(t,
		map[string]string{"content": "foto", "file_type": "image"},
		"attachments[]", "p.jpg", "image/jpeg", []byte("jpegbytes"))
	req := httptest.NewRequest("POST", "/inbound/channel/api/messages", body)
	req.Header.Set("Content-Type", ct)

	parsed, raw, err := parseMultipartInbound(req)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.Text != "foto" {
		t.Errorf("caption must populate text, got %q", parsed.Text)
	}
	if len(raw) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(raw))
	}
}

// TestParseMultipart_Audio: an audio/webm voice note under attachments[] parses too.
func TestParseMultipart_Audio(t *testing.T) {
	body, ct := buildMultipart(t,
		map[string]string{"content": "", "file_type": "audio"},
		"attachments[]", "voice.webm", "audio/webm", []byte("OpusHead..."))
	req := httptest.NewRequest("POST", "/inbound/channel/api/messages", body)
	req.Header.Set("Content-Type", ct)

	parsed, raw, err := parseMultipartInbound(req)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.Text != "" || len(raw) != 1 {
		t.Fatalf("audio-only parse wrong: text=%q files=%d", parsed.Text, len(raw))
	}
	if raw[0].ContentType != "audio/webm" || raw[0].Filename != "voice.webm" {
		t.Errorf("audio metadata wrong: name=%q ct=%q", raw[0].Filename, raw[0].ContentType)
	}
}

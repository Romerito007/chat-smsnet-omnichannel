package service

import (
	"context"
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	chcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/contracts"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
)

// recordingAttachmentStore is a fake InboundAttachmentStore: it persists nothing but
// records each call and returns a ready conversation attachment, so the inbound flow
// (parse → store → link to message) is exercised without a real storage backend. A
// non-empty failWith makes StoreInbound return that error (to assert propagation).
type recordingAttachmentStore struct {
	calls   int
	last    chcontracts.RawFile
	failErr error
}

func (s *recordingAttachmentStore) StoreInbound(_ context.Context, _, filename, contentType string, data []byte) (conventity.Attachment, error) {
	s.calls++
	s.last = chcontracts.RawFile{Filename: filename, ContentType: contentType, Data: data}
	if s.failErr != nil {
		return conventity.Attachment{}, s.failErr
	}
	return conventity.Attachment{ID: "att1", URL: "http://api/v1/attachments/att1/download", ContentType: contentType, Filename: filename, Size: int64(len(data))}, nil
}

func rawImageMsg(ext, caption string) chcontracts.InboundMessage {
	m := inMsg(ext)
	m.Text = caption
	m.RawAttachments = []chcontracts.RawFile{{Filename: "photo.jpg", ContentType: "image/jpeg", Data: []byte("\xff\xd8\xff\xe0jpeg")}}
	return m
}

// TestInbound_MediaOnly_PersistsAttachment is the inbound-media happy path: an image
// with NO caption is accepted (not rejected for empty text), stored, linked to the
// message, and the message is typed as a file.
func TestInbound_MediaOnly_PersistsAttachment(t *testing.T) {
	fx := newInboundFixture()
	store := &recordingAttachmentStore{}
	fx.svc.SetAttachmentStore(store)

	res, err := fx.svc.Handle(tenantCtx(), conn(""), rawImageMsg("ext-img", "")) // content=""
	if err != nil {
		t.Fatalf("media-only inbound must succeed, got: %v", err)
	}
	if store.calls != 1 || store.last.ContentType != "image/jpeg" {
		t.Fatalf("attachment not stored: calls=%d last=%+v", store.calls, store.last)
	}
	msg := fx.msgs.items[lastMessageID(fx)]
	if msg == nil || len(msg.Attachments) != 1 || msg.Attachments[0].ID != "att1" {
		t.Fatalf("message must carry the stored attachment, got %+v", msg)
	}
	if msg.MessageType != conventity.MessageFile {
		t.Errorf("media-only message must be typed as file, got %q", msg.MessageType)
	}
	if res.ConversationID == "" {
		t.Error("expected a conversation id in the result")
	}
}

// TestInbound_MediaWithCaption keeps the caption text alongside the attachment.
func TestInbound_MediaWithCaption(t *testing.T) {
	fx := newInboundFixture()
	fx.svc.SetAttachmentStore(&recordingAttachmentStore{})
	if _, err := fx.svc.Handle(tenantCtx(), conn(""), rawImageMsg("ext-cap", "foto")); err != nil {
		t.Fatalf("media+caption inbound must succeed, got: %v", err)
	}
	msg := fx.msgs.items[lastMessageID(fx)]
	if msg.Text != "foto" || len(msg.Attachments) != 1 {
		t.Errorf("expected caption text + attachment, got text=%q attachments=%d", msg.Text, len(msg.Attachments))
	}
}

// TestInbound_Audio accepts an audio/webm voice note.
func TestInbound_Audio(t *testing.T) {
	fx := newInboundFixture()
	store := &recordingAttachmentStore{}
	fx.svc.SetAttachmentStore(store)
	m := inMsg("ext-audio")
	m.Text = ""
	m.RawAttachments = []chcontracts.RawFile{{Filename: "voice.webm", ContentType: "audio/webm", Data: []byte("OpusHead")}}
	if _, err := fx.svc.Handle(tenantCtx(), conn(""), m); err != nil {
		t.Fatalf("audio inbound must succeed, got: %v", err)
	}
	if store.last.ContentType != "audio/webm" {
		t.Errorf("audio not stored with its content type, got %q", store.last.ContentType)
	}
}

// TestInbound_AttachmentValidationErrorPropagates: a validation error from the store
// (e.g. disallowed type / oversize) must surface as a 4xx validation error, not get
// masked — the controller maps it to 400, never a 500.
func TestInbound_AttachmentValidationErrorPropagates(t *testing.T) {
	fx := newInboundFixture()
	fx.svc.SetAttachmentStore(&recordingAttachmentStore{failErr: apperror.Validation("invalid attachment")})
	_, err := fx.svc.Handle(tenantCtx(), conn(""), rawImageMsg("ext-bad", ""))
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("a store validation error must propagate as validation (→400), got %v", err)
	}
}

// lastMessageID returns the id of the most recently created message in the fixture.
func lastMessageID(fx inboundFixture) string {
	fx.msgs.mu.Lock()
	defer fx.msgs.mu.Unlock()
	if len(fx.msgs.order) == 0 {
		return ""
	}
	return fx.msgs.order[len(fx.msgs.order)-1]
}

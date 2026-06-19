package service

import (
	"context"
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	chcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/contracts"
	convcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
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
	// image/jpeg must be typed as image (renderable), not file/text — otherwise the
	// dashboard shows an empty bubble instead of the picture.
	if msg.MessageType != conventity.MessageImage {
		t.Errorf("media-only image must be typed as image, got %q", msg.MessageType)
	}
	if res.ConversationID == "" {
		t.Error("expected a conversation id in the result")
	}

	// The realtime message_created payload the dashboard receives must carry the
	// attachment AND the renderable type — this is the live path that drew the
	// empty bubble before the fix.
	p, ok := fx.pub.lastPayload(convcontracts.RealtimeMessageCreated).(convcontracts.MessagePayload)
	if !ok {
		t.Fatalf("expected a MessagePayload on the realtime event, got %T", fx.pub.lastPayload(convcontracts.RealtimeMessageCreated))
	}
	if p.MessageType != string(conventity.MessageImage) {
		t.Errorf("realtime message_type = %q, want image", p.MessageType)
	}
	if len(p.Attachments) != 1 || p.Attachments[0].ContentType != "image/jpeg" || p.Attachments[0].URL == "" {
		t.Errorf("realtime payload must include the hydrated attachment, got %+v", p.Attachments)
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
	// A caption must NOT downgrade the type to text — the image still renders.
	if msg.MessageType != conventity.MessageImage {
		t.Errorf("captioned image must stay typed as image, got %q", msg.MessageType)
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
	if msg := fx.msgs.items[lastMessageID(fx)]; msg.MessageType != conventity.MessageAudio {
		t.Errorf("audio/webm must be typed as audio, got %q", msg.MessageType)
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

// TestInbound_Contact: an inbound contact is typed contact, stored, and surfaced on
// the realtime payload.
func TestInbound_Contact(t *testing.T) {
	fx := newInboundFixture()
	m := inMsg("ext-contact")
	m.Text = ""
	m.Contacts = []conventity.ContactCard{{
		Name:   conventity.ContactName{Formatted: "João"},
		Phones: []conventity.ContactPhone{{Phone: "+5511777776666"}},
	}}
	res, err := fx.svc.Handle(tenantCtx(), conn(""), m)
	if err != nil {
		t.Fatalf("inbound contact: %v", err)
	}
	msg := fx.msgs.items[lastMessageID(fx)]
	if msg.MessageType != conventity.MessageContact || len(msg.Contacts) != 1 {
		t.Fatalf("inbound contact not typed/stored: %+v", msg)
	}
	if p, ok := fx.pub.lastPayload(convcontracts.RealtimeMessageCreated).(convcontracts.MessagePayload); !ok || len(p.Contacts) != 1 {
		t.Errorf("realtime payload must carry contacts")
	}
	_ = res
}

// TestInbound_Location: an inbound location is typed location and stored.
func TestInbound_Location(t *testing.T) {
	fx := newInboundFixture()
	m := inMsg("ext-loc")
	m.Text = ""
	m.Location = &conventity.Location{Latitude: -23.56, Longitude: -46.64, Name: "Cliente"}
	if _, err := fx.svc.Handle(tenantCtx(), conn(""), m); err != nil {
		t.Fatalf("inbound location: %v", err)
	}
	msg := fx.msgs.items[lastMessageID(fx)]
	if msg.MessageType != conventity.MessageLocation || msg.Location == nil || msg.Location.Name != "Cliente" {
		t.Fatalf("inbound location not typed/stored: %+v", msg)
	}
}

// TestInbound_Location_InvalidRejected: a malformed inbound location is a 4xx, not a
// corrupt stored message.
func TestInbound_Location_InvalidRejected(t *testing.T) {
	fx := newInboundFixture()
	m := inMsg("ext-badloc")
	m.Location = &conventity.Location{Latitude: 999, Longitude: 0}
	_, err := fx.svc.Handle(tenantCtx(), conn(""), m)
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("expected validation error for bad location, got %v", err)
	}
}

// TestInbound_InteractiveReply: a customer's menu choice is typed interactive_reply,
// mirrors title to text, and resolves context_external_id to the internal menu id.
func TestInbound_InteractiveReply(t *testing.T) {
	fx := newInboundFixture()
	// Seed the menu message we "sent" (outbound) with a known external id (wamid).
	res0, err := fx.svc.Handle(tenantCtx(), conn(""), inMsg("seed")) // opens the conversation
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	convID := res0.ConversationID
	menu := &conventity.Message{ID: "menu1", TenantID: "t1", ConversationID: convID, ExternalMessageID: "wamid.menu", MessageType: conventity.MessageInteractive}
	_ = fx.msgs.Create(tenantCtx(), menu)

	m := inMsg("ext-reply")
	m.Text = ""
	m.InteractiveReply = &chcontracts.InboundInteractiveReply{
		Type: "button_reply", ID: "intent_500mb", Title: "Plano 500MB", ContextExternalID: "wamid.menu",
	}
	if _, err := fx.svc.Handle(tenantCtx(), conn(""), m); err != nil {
		t.Fatalf("inbound reply: %v", err)
	}
	msg := fx.msgs.items[lastMessageID(fx)]
	if msg.MessageType != conventity.MessageInteractiveReply || msg.InteractiveReply == nil {
		t.Fatalf("reply not typed/stored: %+v", msg)
	}
	// The id is a structured, queryable field (the automation trigger), not text.
	if msg.InteractiveReply.ID != "intent_500mb" || msg.InteractiveReply.Type != "button_reply" {
		t.Errorf("reply id/type not stored structurally: %+v", msg.InteractiveReply)
	}
	if msg.Text != "Plano 500MB" {
		t.Errorf("title must mirror to text for the inbox, got %q", msg.Text)
	}
	if msg.InteractiveReply.ContextMessageID != "menu1" {
		t.Errorf("context must resolve to the internal menu id, got %q", msg.InteractiveReply.ContextMessageID)
	}
}

package service

import (
	"context"
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// fakeAttachmentResolver stands in for the attachments service: byID holds the
// hydrated metadata; ready marks which ids pass send-time validation.
type fakeAttachmentResolver struct {
	byID  map[string]entity.Attachment
	ready map[string]bool
}

func (f fakeAttachmentResolver) HydrateAttachments(_ context.Context, ids []string) (map[string]entity.Attachment, error) {
	out := map[string]entity.Attachment{}
	for _, id := range ids {
		if a, ok := f.byID[id]; ok {
			out[id] = a
		}
	}
	return out, nil
}

func (f fakeAttachmentResolver) ValidateMessageAttachments(_ context.Context, ids []string) error {
	for _, id := range ids {
		if !f.ready[id] {
			return apperror.Validation("attachment not found").
				WithDetails(map[string]any{"attachments": "unknown attachment " + id})
		}
	}
	return nil
}

func imageAudioResolver() fakeAttachmentResolver {
	return fakeAttachmentResolver{
		ready: map[string]bool{"img": true, "aud": true},
		byID: map[string]entity.Attachment{
			"img": {ID: "img", URL: "http://api/v1/attachments/img/download", ContentType: "image/jpeg", Filename: "p.jpg", Size: 1200},
			"aud": {ID: "aud", URL: "http://api/v1/attachments/aud/download", ContentType: "audio/mpeg", Filename: "a.mp3", Size: 900},
		},
	}
}

// GET .../messages must return attachments with url/content_type/filename/size
// filled, and derive message_type from the attachment when there is no text.
func TestListMessages_HydratesAttachmentsAndDerivesType(t *testing.T) {
	svc, _, mr, _, _ := newService(map[string]string{"s1": "t1"})
	svc.SetAttachmentResolver(imageAudioResolver())
	id := openConv(t, svc)

	// Stored id-only (the bug shape), no text.
	mr.items = append(mr.items,
		&entity.Message{ID: "m1", TenantID: "t1", ConversationID: id, MessageType: entity.MessageText, Attachments: []entity.Attachment{{ID: "img"}}},
		&entity.Message{ID: "m2", TenantID: "t1", ConversationID: id, MessageType: entity.MessageText, Attachments: []entity.Attachment{{ID: "aud"}}},
	)

	msgs, err := svc.ListMessages(adminCtx(), id, shared.PageRequest{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	byID := map[string]*entity.Message{}
	for _, m := range msgs {
		byID[m.ID] = m
	}
	m1 := byID["m1"]
	if m1 == nil || len(m1.Attachments) != 1 {
		t.Fatalf("m1 missing attachment: %+v", m1)
	}
	a := m1.Attachments[0]
	if a.URL != "http://api/v1/attachments/img/download" || a.ContentType != "image/jpeg" || a.Filename != "p.jpg" || a.Size != 1200 {
		t.Errorf("image attachment not hydrated: %+v", a)
	}
	if m1.MessageType != entity.MessageImage {
		t.Errorf("image message_type = %q, want image", m1.MessageType)
	}
	if byID["m2"].MessageType != entity.MessageAudio {
		t.Errorf("audio message_type = %q, want audio", byID["m2"].MessageType)
	}
}

// TestListMessages_CaptionedImageKeepsImageType locks the captioned-media contract
// on the read path: when a media message ALSO has text (a caption), deriveMessageType
// defers to the STORED type, so the producer must store "image" (not "text"). An
// inbound captioned image stored as text used to render as a plain text bubble.
func TestListMessages_CaptionedImageKeepsImageType(t *testing.T) {
	svc, _, mr, _, _ := newService(map[string]string{"s1": "t1"})
	svc.SetAttachmentResolver(imageAudioResolver())
	id := openConv(t, svc)

	mr.items = append(mr.items,
		&entity.Message{ID: "m1", TenantID: "t1", ConversationID: id, Text: "olha a foto",
			MessageType: entity.MessageImage, Attachments: []entity.Attachment{{ID: "img"}}},
	)

	msgs, err := svc.ListMessages(adminCtx(), id, shared.PageRequest{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var m1 *entity.Message
	for _, m := range msgs {
		if m.ID == "m1" {
			m1 = m
		}
	}
	if m1 == nil {
		t.Fatal("m1 not returned")
	}
	if m1.MessageType != entity.MessageImage {
		t.Errorf("captioned image must stay image on read, got %q", m1.MessageType)
	}
	if m1.Text != "olha a foto" || len(m1.Attachments) != 1 || m1.Attachments[0].ContentType != "image/jpeg" {
		t.Errorf("captioned image must keep its caption + hydrated attachment: %+v", m1)
	}
}

// SendMessage rejects an unknown/unconfirmed attachment and, for a valid image
// attachment with no text, returns a hydrated, image-typed message.
func TestSendMessage_ValidatesAndDerivesType(t *testing.T) {
	svc, _, _, _, _ := newService(map[string]string{"s1": "t1"})
	svc.SetAttachmentResolver(imageAudioResolver())
	id := openConv(t, svc)

	if _, err := svc.SendMessage(adminCtx(), id, contracts.SendMessage{
		Attachments: []entity.Attachment{{ID: "missing"}},
	}); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("unknown attachment must be rejected, got %v", err)
	}

	msg, err := svc.SendMessage(adminCtx(), id, contracts.SendMessage{
		Attachments: []entity.Attachment{{ID: "img"}},
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if msg.MessageType != entity.MessageImage {
		t.Errorf("message_type = %q, want image", msg.MessageType)
	}
	if len(msg.Attachments) != 1 || msg.Attachments[0].ContentType != "image/jpeg" || msg.Attachments[0].URL == "" {
		t.Errorf("returned message not hydrated: %+v", msg.Attachments)
	}
}

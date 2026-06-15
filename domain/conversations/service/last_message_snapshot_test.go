package service

import (
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
)

// TestSnapshot_SendMessageDenormalizes verifies the inbox preview is written onto
// the conversation document at the outbound chokepoint (persistMessage), so the
// inbox never aggregates the messages collection.
func TestSnapshot_SendMessageDenormalizes(t *testing.T) {
	svc, cr, _, _, _ := newService(map[string]string{"s1": "t1"})
	id := openConv(t, svc)

	if _, err := svc.SendMessage(adminCtx(), id, contracts.SendMessage{Text: "olá mundo"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	conv := cr.items[id]
	if conv.LastMessage == nil {
		t.Fatal("conversation must carry a denormalized last_message snapshot")
	}
	if conv.LastMessage.Preview != "olá mundo" || conv.LastMessage.SenderType != entity.SenderAgent {
		t.Errorf("snapshot mismatch: %+v", conv.LastMessage)
	}
	if conv.LastMessage.MessageType != entity.MessageText {
		t.Errorf("snapshot message_type = %q, want text", conv.LastMessage.MessageType)
	}
}

// TestSnapshot_EditLastMessageRefreshesPreview: editing the conversation's latest
// message refreshes the denormalized preview.
func TestSnapshot_EditLastMessageRefreshesPreview(t *testing.T) {
	svc, cr, _, _, _ := newService(map[string]string{"s1": "t1"})
	id := openConv(t, svc)
	msg, err := svc.SendMessage(adminCtx(), id, contracts.SendMessage{Text: "original"})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if _, err := svc.EditMessage(adminCtx(), id, msg.ID, contracts.EditMessage{Text: "editado"}); err != nil {
		t.Fatalf("edit: %v", err)
	}
	if got := cr.items[id].LastMessage; got == nil || got.Preview != "editado" {
		t.Errorf("editing the last message must refresh the preview, got %+v", got)
	}
}

// TestSnapshot_DeleteLastMessageRecomputes: deleting the conversation's latest
// message falls back to the previous one (via the indexed LatestByConversation).
func TestSnapshot_DeleteLastMessageRecomputes(t *testing.T) {
	svc, cr, _, _, _ := newService(map[string]string{"s1": "t1"})
	id := openConv(t, svc)
	if _, err := svc.SendMessage(adminCtx(), id, contracts.SendMessage{Text: "primeira"}); err != nil {
		t.Fatalf("send 1: %v", err)
	}
	m2, err := svc.SendMessage(adminCtx(), id, contracts.SendMessage{Text: "segunda"})
	if err != nil {
		t.Fatalf("send 2: %v", err)
	}
	if cr.items[id].LastMessage.Preview != "segunda" {
		t.Fatalf("precondition: snapshot should be the latest message")
	}
	if err := svc.DeleteMessage(adminCtx(), id, m2.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got := cr.items[id].LastMessage; got == nil || got.Preview != "primeira" {
		t.Errorf("delete-of-last must recompute to the previous message, got %+v", got)
	}
}

// TestSnapshot_DeleteOnlyMessageClears: deleting the sole message clears the
// snapshot (no message left to preview).
func TestSnapshot_DeleteOnlyMessageClears(t *testing.T) {
	svc, cr, _, _, _ := newService(map[string]string{"s1": "t1"})
	id := openConv(t, svc)
	m, err := svc.SendMessage(adminCtx(), id, contracts.SendMessage{Text: "única"})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if err := svc.DeleteMessage(adminCtx(), id, m.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got := cr.items[id].LastMessage; got != nil {
		t.Errorf("deleting the only message must clear the snapshot, got %+v", got)
	}
}

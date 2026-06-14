package service

import (
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// sendAs creates a conversation and an agent message authored by userID.
func sendAs(t *testing.T, svc *Service, userID string) (convID, msgID string) {
	t.Helper()
	ctx := actorCtx("t1", userID, authz.ScopeAll, nil)
	conv, err := svc.Create(ctx, contracts.CreateConversation{ContactID: "c1", ChannelID: "ch1", SectorID: "s1"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	msg, err := svc.SendMessage(ctx, conv.ID, contracts.SendMessage{MessageType: entity.MessageText, Text: "original"})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	return conv.ID, msg.ID
}

func TestEditMessage_SetsEditedAtKeepsHistoryAndPublishes(t *testing.T) {
	svc, _, mr, er, pub := newService(map[string]string{"s1": "t1"})
	convID, msgID := sendAs(t, svc, "u1")
	pub.events = nil
	er.items = nil

	ctx := actorCtx("t1", "u1", authz.ScopeAll, nil)
	edited, err := svc.EditMessage(ctx, convID, msgID, contracts.EditMessage{Text: "  corrected  "})
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	if edited.Text != "corrected" {
		t.Errorf("text = %q, want trimmed 'corrected'", edited.Text)
	}
	if edited.EditedAt == nil {
		t.Errorf("edited_at must be set")
	}
	// History preserved: the message still exists (not deleted) and remains listable.
	if got := mr.items[0]; got.DeletedAt != nil {
		t.Errorf("edit must not delete the message")
	}
	if !published(pub, contracts.RealtimeMessageUpdated) {
		t.Errorf("expected message.updated, got %+v", pub.events)
	}
	if er.items[len(er.items)-1].Type != entity.EventMessageEdited {
		t.Errorf("expected a message.edited timeline event, got %+v", er.items)
	}
}

func TestEditMessage_NonAuthorWithoutPermissionForbidden(t *testing.T) {
	svc, _, _, _, _ := newService(map[string]string{"s1": "t1"})
	convID, msgID := sendAs(t, svc, "u1")

	// u2 is not the author and lacks message.delete.
	ctx := actorCtx("t1", "u2", authz.ScopeAll, nil)
	_, err := svc.EditMessage(ctx, convID, msgID, contracts.EditMessage{Text: "hax"})
	if apperror.From(err).Code != apperror.CodeForbidden {
		t.Errorf("expected forbidden for non-author edit, got %v", err)
	}
}

func TestEditMessage_NonAuthorWithMessageDeleteAllowed(t *testing.T) {
	svc, _, _, _, pub := newService(map[string]string{"s1": "t1"})
	convID, msgID := sendAs(t, svc, "u1")
	pub.events = nil

	// u2 holds message.delete (elevated "manage messages").
	ctx := actorCtx("t1", "u2", authz.ScopeAll, nil, authz.MessageDelete)
	if _, err := svc.EditMessage(ctx, convID, msgID, contracts.EditMessage{Text: "fixed by supervisor"}); err != nil {
		t.Fatalf("supervisor edit: %v", err)
	}
	if !published(pub, contracts.RealtimeMessageUpdated) {
		t.Errorf("expected message.updated, got %+v", pub.events)
	}
}

func TestEditMessage_CustomerMessageRejected(t *testing.T) {
	svc, _, mr, _, _ := newService(map[string]string{"s1": "t1"})
	convID, _ := sendAs(t, svc, "u1")
	// Inject an inbound (customer) message into the conversation.
	mr.items = append(mr.items, &entity.Message{
		ID: "cust1", TenantID: "t1", ConversationID: convID,
		SenderType: entity.SenderCustomer, Direction: entity.DirectionInbound,
		MessageType: entity.MessageText, Text: "hi", CreatedAt: time.Now(),
	})
	ctx := actorCtx("t1", "u1", authz.ScopeAll, nil, authz.MessageDelete)
	_, err := svc.EditMessage(ctx, convID, "cust1", contracts.EditMessage{Text: "rewrite"})
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("editing a customer message must be rejected, got %v", err)
	}
}

func TestDeleteMessage_SoftDeleteHidesFromListAndPublishes(t *testing.T) {
	svc, _, mr, er, pub := newService(map[string]string{"s1": "t1"})
	convID, msgID := sendAs(t, svc, "u1")
	pub.events = nil
	er.items = nil

	ctx := actorCtx("t1", "u9", authz.ScopeAll, nil, authz.MessageDelete)
	if err := svc.DeleteMessage(ctx, convID, msgID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	// Still in the DB (history), but soft-deleted.
	if mr.items[0].DeletedAt == nil {
		t.Errorf("deleted_at must be set (soft delete preserves the row)")
	}
	// Disappears from listings.
	list, _ := svc.ListMessages(ctx, convID, shared.PageRequest{Limit: 50})
	for _, m := range list {
		if m.ID == msgID {
			t.Errorf("deleted message must not appear in listings")
		}
	}
	if !published(pub, contracts.RealtimeMessageDeleted) {
		t.Errorf("expected message.deleted, got %+v", pub.events)
	}
	if er.items[len(er.items)-1].Type != entity.EventMessageDeleted {
		t.Errorf("expected a message.deleted timeline event, got %+v", er.items)
	}
}

func TestDeleteMessage_Idempotent(t *testing.T) {
	svc, _, _, _, _ := newService(map[string]string{"s1": "t1"})
	convID, msgID := sendAs(t, svc, "u1")
	ctx := actorCtx("t1", "u9", authz.ScopeAll, nil, authz.MessageDelete)
	if err := svc.DeleteMessage(ctx, convID, msgID); err != nil {
		t.Fatalf("first delete: %v", err)
	}
	if err := svc.DeleteMessage(ctx, convID, msgID); err != nil {
		t.Errorf("second delete must be a no-op, got %v", err)
	}
}

func TestDeleteMessage_AuthorMayDeleteOwn(t *testing.T) {
	svc, _, mr, _, _ := newService(map[string]string{"s1": "t1"})
	convID, msgID := sendAs(t, svc, "u1")
	// Author without message.delete (service-level check; the route still gates it).
	ctx := actorCtx("t1", "u1", authz.ScopeAll, nil)
	if err := svc.DeleteMessage(ctx, convID, msgID); err != nil {
		t.Fatalf("author delete: %v", err)
	}
	if mr.items[0].DeletedAt == nil {
		t.Errorf("author's own message should be soft-deleted")
	}
}

func TestDeleteMessage_NonAuthorWithoutPermissionForbidden(t *testing.T) {
	svc, _, _, _, _ := newService(map[string]string{"s1": "t1"})
	convID, msgID := sendAs(t, svc, "u1")
	ctx := actorCtx("t1", "u2", authz.ScopeAll, nil) // not author, no message.delete
	if err := svc.DeleteMessage(ctx, convID, msgID); apperror.From(err).Code != apperror.CodeForbidden {
		t.Errorf("expected forbidden, got %v", err)
	}
}

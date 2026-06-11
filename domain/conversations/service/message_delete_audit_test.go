package service

import (
	"context"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// fakeAuditor captures audit entries for assertions.
type fakeAuditor struct{ entries []shared.AuditEntry }

func (a *fakeAuditor) Record(_ context.Context, e shared.AuditEntry) error {
	a.entries = append(a.entries, e)
	return nil
}

func (a *fakeAuditor) find(action string) (shared.AuditEntry, bool) {
	for _, e := range a.entries {
		if e.Action == action {
			return e, true
		}
	}
	return shared.AuditEntry{}, false
}

func TestDeleteMessage_RecordsAuditWithSenderType(t *testing.T) {
	svc, _, _, _, _ := newService(map[string]string{"s1": "t1"})
	aud := &fakeAuditor{}
	svc.SetAuditor(aud)
	convID, msgID := sendAs(t, svc, "u1") // agent-authored message

	if err := svc.DeleteMessage(actorCtx("t1", "u1", authz.ScopeAll, nil), convID, msgID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	e, ok := aud.find("message.deleted")
	if !ok {
		t.Fatalf("expected a message.deleted audit entry, got %+v", aud.entries)
	}
	if e.ResourceType != "message" || e.ResourceID != msgID {
		t.Errorf("unexpected audit resource: %+v", e)
	}
	if e.Data["sender_type"] != string(entity.SenderAgent) {
		t.Errorf("sender_type = %v, want agent", e.Data["sender_type"])
	}
}

func TestDeleteMessage_CustomerMessageAuditedForModeration(t *testing.T) {
	svc, _, mr, _, _ := newService(map[string]string{"s1": "t1"})
	aud := &fakeAuditor{}
	svc.SetAuditor(aud)
	convID, _ := sendAs(t, svc, "u1")
	// A customer (inbound) message moderated by a supervisor with message.delete.
	mr.items = append(mr.items, &entity.Message{
		ID: "cust1", TenantID: "t1", ConversationID: convID,
		SenderType: entity.SenderCustomer, Direction: entity.DirectionInbound,
		MessageType: entity.MessageText, Text: "spam", CreatedAt: time.Now(),
	})

	ctx := actorCtx("t1", "mod1", authz.ScopeAll, nil, authz.MessageDelete)
	if err := svc.DeleteMessage(ctx, convID, "cust1"); err != nil {
		t.Fatalf("delete customer message: %v", err)
	}

	e, ok := aud.find("message.deleted")
	if !ok {
		t.Fatalf("customer-message deletion must be audited, got %+v", aud.entries)
	}
	if e.Data["sender_type"] != string(entity.SenderCustomer) {
		t.Errorf("sender_type = %v, want customer (content moderation)", e.Data["sender_type"])
	}
	if e.ResourceID != "cust1" {
		t.Errorf("resource_id = %q, want cust1", e.ResourceID)
	}
}

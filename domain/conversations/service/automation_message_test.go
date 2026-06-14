package service

import (
	"context"
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// capturingSink records the origin carried on the context for each emitted rule
// event, so a test can assert the anti-loop origin derivation.
type capturingSink struct {
	origins []shared.RuleOrigin
	events  []string
}

func (s *capturingSink) EmitRuleEvent(ctx context.Context, _, event, _ string, _ any) {
	s.origins = append(s.origins, shared.RuleOriginFromContext(ctx))
	s.events = append(s.events, event)
}

// TestSendAutomationMessage_DerivesAutomationOrigin proves the real pipeline tags
// the message_created rule event as origin=automation (derived from the automation
// sender), which is what the evaluator suppresses — closing the anti-loop.
func TestSendAutomationMessage_DerivesAutomationOrigin(t *testing.T) {
	svc, cr, mr, _, _ := newService(map[string]string{"s1": "t1"})
	cr.items["cv1"] = &entity.Conversation{ID: "cv1", TenantID: "t1", ContactID: "c1", ChannelID: "ch1", Channel: "whatsapp", SectorID: "s1", Status: entity.StatusAssigned}
	sink := &capturingSink{}
	svc.SetRuleEventSink(sink)

	if err := svc.SendAutomationMessage(shared.WithTenant(context.Background(), "t1"), "cv1", "r1", "olá do robô"); err != nil {
		t.Fatalf("send automation message: %v", err)
	}

	// The message is persisted as SenderType=automation, SenderID=rule id.
	if len(mr.items) != 1 {
		t.Fatalf("expected one persisted message, got %d", len(mr.items))
	}
	var got *entity.Message
	for _, m := range mr.items {
		got = m
	}
	if got.SenderType != entity.SenderAutomation || got.SenderID != "r1" {
		t.Errorf("expected automation sender (id r1), got type=%q id=%q", got.SenderType, got.SenderID)
	}
	// The emitted message_created rule event carries origin=automation.
	if len(sink.origins) != 1 || sink.origins[0] != shared.OriginAutomation {
		t.Fatalf("expected one rule event with origin=automation, got %+v (events %v)", sink.origins, sink.events)
	}
}

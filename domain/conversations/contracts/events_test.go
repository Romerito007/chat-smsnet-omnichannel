package contracts

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
)

// TestIntegrationMessagePayload_CarriesContactAndAgent verifies the outbound-webhook
// message payload carries the recipient contact (with its channel identities — the
// gateway's routing key) and the agent block, while the lean realtime payload omits
// them entirely.
func TestIntegrationMessagePayload_CarriesContactAndAgent(t *testing.T) {
	msg := &entity.Message{
		ID: "m1", ConversationID: "c1", SenderType: entity.SenderAgent, SenderID: "u9",
		Direction: entity.DirectionOutbound, MessageType: entity.MessageText, Text: "hi",
	}

	// Lean realtime payload: no enrichment blocks at all.
	lean := NewMessagePayload(msg)
	if lean.Contact != nil || lean.Agent != nil || lean.Conversation != nil {
		t.Fatalf("lean payload must omit enrichment blocks: %+v", lean)
	}
	if b, _ := json.Marshal(lean); strings.Contains(string(b), `"contact":`) || strings.Contains(string(b), `"agent":`) {
		t.Errorf("lean JSON leaked enrichment blocks: %s", b)
	}

	// Integration payload enriched by the caller (as emitMessageWebhook does).
	p := NewIntegrationMessagePayload(msg, nil)
	p.Contact = &WebhookContact{
		ID: "ct1", Name: "Alice", Phone: "+5511999",
		Identities:       []WebhookIdentity{{Channel: "whatsapp", ExternalID: "5511999@s.whatsapp.net"}},
		CustomAttributes: map[string]any{"plan": "gold"},
	}
	p.Agent = &WebhookAgent{ID: "u9", Name: "Bob"}
	p.Conversation = &WebhookConversationRef{CustomAttributes: map[string]any{"ticket": "42"}}

	b, _ := json.Marshal(p)
	js := string(b)
	for _, want := range []string{
		`"channel":"whatsapp"`, `"external_id":"5511999@s.whatsapp.net"`,
		`"phone":"+5511999"`, `"plan":"gold"`, `"agent":`, `"name":"Bob"`, `"ticket":"42"`,
	} {
		if !strings.Contains(js, want) {
			t.Errorf("integration JSON missing %q: %s", want, js)
		}
	}
}

// TestIntegrationConversationPayload_ContactAndAssignedAgent verifies the conversation
// webhook payload embeds custom_attributes, the recipient contact and the assigned
// agent, and that a nil agent (unassigned / inbound) is omitted.
func TestIntegrationConversationPayload_ContactAndAssignedAgent(t *testing.T) {
	conv := &entity.Conversation{
		ID: "c1", TenantID: "t1", ContactID: "ct1", Channel: "whatsapp",
		Status: entity.StatusAssigned, AssignedTo: "u9",
		CustomAttributes: map[string]any{"vip": true},
	}
	contact := &WebhookContact{ID: "ct1", Name: "Alice"}

	withAgent := NewIntegrationConversationPayload(conv, contact, &WebhookAgent{ID: "u9", Name: "Bob"})
	if withAgent.ID != "c1" || withAgent.Contact == nil || withAgent.AssignedAgent == nil {
		t.Fatalf("expected embedded conversation + contact + agent: %+v", withAgent)
	}
	if js := mustJSON(t, withAgent); !strings.Contains(js, `"vip":true`) || !strings.Contains(js, `"assigned_agent"`) {
		t.Errorf("missing custom_attributes or assigned_agent: %s", js)
	}

	// Unassigned (or inbound, where agents aren't resolved) → assigned_agent omitted.
	noAgent := NewIntegrationConversationPayload(conv, contact, nil)
	if js := mustJSON(t, noAgent); strings.Contains(js, `"assigned_agent"`) {
		t.Errorf("assigned_agent must be omitted when nil: %s", js)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

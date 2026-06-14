package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	arcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/automationrules/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/automationrules/entity"
	contactentity "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/entity"
	contactrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/repository"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	convrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// fakeConvRepo embeds the interface and overrides FindByID.
type fakeConvRepo struct {
	convrepo.ConversationRepository
	conv *conventity.Conversation
}

func (r *fakeConvRepo) FindByID(context.Context, string) (*conventity.Conversation, error) {
	return r.conv, nil
}

// fakeContactRepo embeds the interface and overrides FindByID.
type fakeContactRepo struct {
	contactrepo.ContactRepository
	contact *contactentity.Contact
}

func (r *fakeContactRepo) FindByID(context.Context, string) (*contactentity.Contact, error) {
	return r.contact, nil
}

type emitCall struct {
	webhookID string
	event     string
	data      string
}

type fakeEmitter struct{ calls []emitCall }

func (e *fakeEmitter) EmitTo(_ context.Context, _, webhookID, event string, payload any) error {
	raw, _ := json.Marshal(payload)
	e.calls = append(e.calls, emitCall{webhookID: webhookID, event: event, data: string(raw)})
	return nil
}

type fakeDeduper struct{ allow bool }

func (d fakeDeduper) Allow(context.Context, string) (bool, error) { return d.allow, nil }

func newEvaluator(conv *conventity.Conversation, contact *contactentity.Contact, rules *fakeRuleRepo, emitter *fakeEmitter, allow bool) (*Evaluator, *fakeLogRepo) {
	logs := &fakeLogRepo{}
	ev := NewEvaluator(
		rules, logs,
		&fakeConvRepo{conv: conv},
		&fakeContactRepo{contact: contact},
		NewExecutor(emitter, nil),
		fakeDeduper{allow: allow},
		fixedClock{t: time.Unix(1700000000, 0).UTC()},
	)
	return ev, logs
}

func ruleFor(event entity.RuleEvent, webhookID string, conds ...entity.Condition) *entity.AutomationRule {
	return &entity.AutomationRule{
		ID: "r1", TenantID: "t1", Name: "R", Event: event, Enabled: true,
		Conditions: conds,
		Actions:    []entity.Action{{Type: entity.ActionSendWebhook, Params: map[string]string{"webhook_id": webhookID}}},
	}
}

// fakeMessenger records automation message sends. When loopback is set it
// re-enters the evaluator with an origin=automation message.created task — exactly
// what the conversations service would emit for an automation-authored message —
// so a test can prove the loop terminates.
type fakeMessenger struct {
	ev    *Evaluator
	sends int
}

func (m *fakeMessenger) SendAutomationMessage(ctx context.Context, conversationID, _, _ string) error {
	m.sends++
	if m.ev != nil {
		_ = m.ev.Evaluate(ctx, arcontracts.EvaluateTask{
			TenantID: "t1", Event: "message.created", ConversationID: conversationID,
			EventID: "evt-auto", Origin: string(shared.OriginAutomation),
		})
	}
	return nil
}

func task(event, convID string, data any) arcontracts.EvaluateTask {
	raw, _ := json.Marshal(data)
	return arcontracts.EvaluateTask{TenantID: "t1", Event: event, ConversationID: convID, Data: raw}
}

func mustJSON(v any) json.RawMessage {
	raw, _ := json.Marshal(v)
	return raw
}

func TestEvaluate_MatchFiresWebhook(t *testing.T) {
	conv := &conventity.Conversation{ID: "cv1", TenantID: "t1", Status: conventity.Status("new"), Channel: "whatsapp", ContactID: "c1"}
	repo := newFakeRuleRepo()
	repo.byID["r1"] = ruleFor(entity.EventConversationCreated, "wh1", entity.Condition{Field: entity.FieldStatus, Operator: entity.OpEqualTo, Value: "new"})
	emitter := &fakeEmitter{}
	ev, logs := newEvaluator(conv, &contactentity.Contact{ID: "c1", Phone: "+55"}, repo, emitter, true)

	if err := ev.Evaluate(context.Background(), task("conversation.created", "cv1", map[string]any{"id": "cv1"})); err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(emitter.calls) != 1 {
		t.Fatalf("expected 1 webhook fire, got %d", len(emitter.calls))
	}
	if emitter.calls[0].webhookID != "wh1" || emitter.calls[0].event != "conversation_created" {
		t.Errorf("wrong emit: %+v", emitter.calls[0])
	}
	if len(logs.created) != 1 || logs.created[0].Status != entity.EvalActionEnqueued {
		t.Errorf("expected action_enqueued log, got %+v", logs.created)
	}
}

func TestEvaluate_NoMatchDoesNotFire(t *testing.T) {
	conv := &conventity.Conversation{ID: "cv1", TenantID: "t1", Status: conventity.Status("open"), ContactID: "c1"}
	repo := newFakeRuleRepo()
	repo.byID["r1"] = ruleFor(entity.EventConversationCreated, "wh1", entity.Condition{Field: entity.FieldStatus, Operator: entity.OpEqualTo, Value: "new"})
	emitter := &fakeEmitter{}
	ev, _ := newEvaluator(conv, nil, repo, emitter, true)

	_ = ev.Evaluate(context.Background(), task("conversation.created", "cv1", nil))
	if len(emitter.calls) != 0 {
		t.Errorf("no-match must not fire: %+v", emitter.calls)
	}
}

func TestEvaluate_DedupSkips(t *testing.T) {
	conv := &conventity.Conversation{ID: "cv1", TenantID: "t1", Status: conventity.Status("new"), ContactID: "c1"}
	repo := newFakeRuleRepo()
	repo.byID["r1"] = ruleFor(entity.EventConversationCreated, "wh1")
	emitter := &fakeEmitter{}
	ev, logs := newEvaluator(conv, nil, repo, emitter, false) // deduper denies

	_ = ev.Evaluate(context.Background(), task("conversation.created", "cv1", nil))
	if len(emitter.calls) != 0 {
		t.Errorf("dedup must suppress the fire: %+v", emitter.calls)
	}
	if len(logs.created) != 1 || logs.created[0].Status != entity.EvalSkippedDedup {
		t.Errorf("expected skipped_dedup log, got %+v", logs.created)
	}
}

func TestEvaluate_MessageCreatedMatchesAgainstConversation(t *testing.T) {
	// The event payload is a MESSAGE; the condition is on the conversation channel.
	// The worker must hydrate the conversation and match against it.
	conv := &conventity.Conversation{ID: "cv1", TenantID: "t1", Channel: "whatsapp", ContactID: "c1"}
	repo := newFakeRuleRepo()
	repo.byID["r1"] = ruleFor(entity.EventMessageCreated, "wh1", entity.Condition{Field: entity.FieldChannel, Operator: entity.OpEqualTo, Value: "whatsapp"})
	emitter := &fakeEmitter{}
	ev, _ := newEvaluator(conv, nil, repo, emitter, true)

	msgPayload := map[string]any{"id": "m1", "conversation_id": "cv1", "text": "oi"}
	if err := ev.Evaluate(context.Background(), task("message.created", "cv1", msgPayload)); err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(emitter.calls) != 1 {
		t.Fatalf("expected fire via conversation hydration, got %d", len(emitter.calls))
	}
	// The webhook data is the original MESSAGE payload, event is the rule wire name.
	if emitter.calls[0].event != "message_created" {
		t.Errorf("event = %q, want message_created", emitter.calls[0].event)
	}
	var got map[string]any
	_ = json.Unmarshal([]byte(emitter.calls[0].data), &got)
	if got["id"] != "m1" {
		t.Errorf("webhook data should be the message payload, got %s", emitter.calls[0].data)
	}
}

func TestEvaluate_AutomationOriginSkipped(t *testing.T) {
	// An event produced BY an automation action (origin=automation) must never be
	// evaluated — the rule does not fire even though it matches.
	conv := &conventity.Conversation{ID: "cv1", TenantID: "t1", Status: conventity.Status("new"), Channel: "whatsapp", ContactID: "c1"}
	repo := newFakeRuleRepo()
	repo.byID["r1"] = ruleFor(entity.EventMessageCreated, "wh1")
	emitter := &fakeEmitter{}
	ev, _ := newEvaluator(conv, nil, repo, emitter, true)

	err := ev.Evaluate(context.Background(), arcontracts.EvaluateTask{
		TenantID: "t1", Event: "message.created", ConversationID: "cv1",
		EventID: "e1", Origin: string(shared.OriginAutomation),
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(emitter.calls) != 0 {
		t.Errorf("automation-origin event must not fire any rule: %+v", emitter.calls)
	}
}

// TestEvaluate_SendMessageDoesNotLoop is the critical anti-loop case: a rule on
// message_created whose action is send_message. The automation message it creates
// emits its OWN message.created — but that one is origin=automation, so the rule
// does NOT fire again. The send happens exactly once; the loop terminates.
func TestEvaluate_SendMessageDoesNotLoop(t *testing.T) {
	conv := &conventity.Conversation{ID: "cv1", TenantID: "t1", Status: conventity.Status("new"), Channel: "whatsapp", ContactID: "c1"}
	repo := newFakeRuleRepo()
	repo.byID["r1"] = &entity.AutomationRule{
		ID: "r1", TenantID: "t1", Name: "auto-reply", Event: entity.EventMessageCreated, Enabled: true,
		Actions: []entity.Action{{Type: entity.ActionSendMessage, Params: map[string]string{"text": "olá!"}}},
	}
	logs := &fakeLogRepo{}
	messenger := &fakeMessenger{}
	ev := NewEvaluator(
		repo, logs,
		&fakeConvRepo{conv: conv},
		&fakeContactRepo{contact: &contactentity.Contact{ID: "c1"}},
		NewExecutor(&fakeEmitter{}, messenger),
		fakeDeduper{allow: true},
		fixedClock{t: time.Unix(1700000000, 0).UTC()},
	)
	messenger.ev = ev // close the loop: the automation message re-enters the evaluator

	// A genuine (external) customer message arrives.
	err := ev.Evaluate(context.Background(), arcontracts.EvaluateTask{
		TenantID: "t1", Event: "message.created", ConversationID: "cv1",
		EventID: "evt-customer", Origin: string(shared.OriginExternal),
		Data: mustJSON(map[string]any{"id": "m1", "text": "preciso de suporte"}),
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if messenger.sends != 1 {
		t.Fatalf("expected exactly one automation message (no loop), got %d", messenger.sends)
	}
}

func TestEvaluate_UnmappedEventIgnored(t *testing.T) {
	repo := newFakeRuleRepo()
	emitter := &fakeEmitter{}
	ev, _ := newEvaluator(nil, nil, repo, emitter, true)
	if err := ev.Evaluate(context.Background(), task("conversation.assigned", "cv1", nil)); err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(emitter.calls) != 0 {
		t.Errorf("unmapped event must be ignored")
	}
}

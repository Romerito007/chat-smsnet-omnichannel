package service

import (
	"context"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/automationrules/entity"
	contactentity "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/entity"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// recordingOps records the conversation operations the executor invokes.
type recordingOps struct {
	noopConvOps
	calls []string
	msgs  []string
	err   error
}

func (o *recordingOps) SendAutomationMessage(_ context.Context, _, _, text string) error {
	o.calls = append(o.calls, "send_message")
	o.msgs = append(o.msgs, text)
	return o.err
}

func (o *recordingOps) AutomationAddTag(_ context.Context, _, tagID string) error {
	o.calls = append(o.calls, "add_tag:"+tagID)
	return o.err
}

type fakeBudget struct{ allow bool }

func (b fakeBudget) AllowAutomationMessage(context.Context, string) (bool, error) {
	return b.allow, nil
}

func evalWithExecutor(conv *conventity.Conversation, repo *fakeRuleRepo, ops ConversationOps, budget BudgetLimiter) (*Evaluator, *fakeLogRepo) {
	logs := &fakeLogRepo{}
	ev := NewEvaluator(
		repo, logs,
		&fakeConvRepo{conv: conv},
		&fakeContactRepo{contact: &contactentity.Contact{ID: "c1"}},
		NewExecutor(&fakeEmitter{}, ops, budget),
		fakeDeduper{allow: true},
		shared.NoopLocker{},
		fixedClock{t: time.Unix(1700000000, 0).UTC()},
	)
	return ev, logs
}

func sendMessageRule(id string, priority int, conds ...entity.Condition) *entity.AutomationRule {
	return &entity.AutomationRule{
		ID: id, TenantID: "t1", Name: id, Event: entity.EventMessageCreated, Enabled: true, Priority: priority,
		Conditions: conds,
		Actions:    []entity.Action{{Type: entity.ActionSendMessage, Params: map[string]string{"text": id}}},
	}
}

func TestEvaluate_MessageContentCondition(t *testing.T) {
	conv := &conventity.Conversation{ID: "cv1", TenantID: "t1", Channel: "whatsapp", ContactID: "c1"}
	repo := newFakeRuleRepo()
	repo.byID["r1"] = sendMessageRule("r1", 0, entity.Condition{
		Field: entity.FieldMessageContent, Operator: entity.OpContains, Value: "suporte",
	})
	ops := &recordingOps{}
	ev, _ := evalWithExecutor(conv, repo, ops, fakeBudget{allow: true})

	// Customer text contains "suporte" → fires.
	_ = ev.Evaluate(context.Background(), task("message.created", "cv1", map[string]any{"id": "m1", "text": "preciso de SUPORTE agora"}))
	if len(ops.msgs) != 1 {
		t.Fatalf("expected the rule to fire on matching content, got %d sends", len(ops.msgs))
	}

	// Different message that does not contain it → does not fire.
	ops2 := &recordingOps{}
	ev2, _ := evalWithExecutor(conv, repo, ops2, fakeBudget{allow: true})
	_ = ev2.Evaluate(context.Background(), task("message.created", "cv1", map[string]any{"id": "m2", "text": "bom dia"}))
	if len(ops2.msgs) != 0 {
		t.Errorf("must not fire when content does not match: %+v", ops2.msgs)
	}
}

func TestEvaluate_InternalActionRuns(t *testing.T) {
	conv := &conventity.Conversation{ID: "cv1", TenantID: "t1", Channel: "whatsapp", ContactID: "c1"}
	repo := newFakeRuleRepo()
	repo.byID["r1"] = &entity.AutomationRule{
		ID: "r1", TenantID: "t1", Name: "tagger", Event: entity.EventMessageCreated, Enabled: true,
		Actions: []entity.Action{{Type: entity.ActionAddTag, Params: map[string]string{"tag_id": "vip"}}},
	}
	ops := &recordingOps{}
	ev, logs := evalWithExecutor(conv, repo, ops, fakeBudget{allow: true})

	_ = ev.Evaluate(context.Background(), task("message.created", "cv1", map[string]any{"id": "m1"}))
	if len(ops.calls) != 1 || ops.calls[0] != "add_tag:vip" {
		t.Fatalf("expected add_tag action to run, got %+v", ops.calls)
	}
	if len(logs.created) != 1 || logs.created[0].Status != entity.EvalActionEnqueued || logs.created[0].ActionType != "add_tag" {
		t.Errorf("expected per-action action_enqueued log, got %+v", logs.created)
	}
}

func TestEvaluate_BudgetSuppressesMessage(t *testing.T) {
	conv := &conventity.Conversation{ID: "cv1", TenantID: "t1", Channel: "whatsapp", ContactID: "c1"}
	repo := newFakeRuleRepo()
	repo.byID["r1"] = sendMessageRule("r1", 0)
	ops := &recordingOps{}
	ev, logs := evalWithExecutor(conv, repo, ops, fakeBudget{allow: false}) // fuse tripped

	_ = ev.Evaluate(context.Background(), task("message.created", "cv1", map[string]any{"id": "m1"}))
	if len(ops.msgs) != 0 {
		t.Errorf("budget fuse must suppress the message send: %+v", ops.msgs)
	}
	if len(logs.created) != 1 || logs.created[0].Status != entity.EvalSkippedBudget {
		t.Errorf("expected skipped_budget log, got %+v", logs.created)
	}
}

func TestEvaluate_PriorityOrder(t *testing.T) {
	conv := &conventity.Conversation{ID: "cv1", TenantID: "t1", Channel: "whatsapp", ContactID: "c1"}
	repo := newFakeRuleRepo()
	repo.byID["high"] = sendMessageRule("low-prio-runs-second", 10)
	repo.byID["low"] = sendMessageRule("low-prio-runs-first", 1)
	ops := &recordingOps{}
	ev, _ := evalWithExecutor(conv, repo, ops, fakeBudget{allow: true})

	_ = ev.Evaluate(context.Background(), task("message.created", "cv1", map[string]any{"id": "m1"}))
	if len(ops.msgs) != 2 || ops.msgs[0] != "low-prio-runs-first" || ops.msgs[1] != "low-prio-runs-second" {
		t.Fatalf("rules must run in priority asc order, got %+v", ops.msgs)
	}
}

func TestEvaluate_MissingRefSkipped(t *testing.T) {
	conv := &conventity.Conversation{ID: "cv1", TenantID: "t1", Channel: "whatsapp", ContactID: "c1"}
	repo := newFakeRuleRepo()
	repo.byID["r1"] = &entity.AutomationRule{
		ID: "r1", TenantID: "t1", Name: "tagger", Event: entity.EventMessageCreated, Enabled: true,
		Actions: []entity.Action{{Type: entity.ActionAddTag, Params: map[string]string{"tag_id": "ghost"}}},
	}
	ops := &recordingOps{err: apperror.NotFound("tag not found")}
	ev, logs := evalWithExecutor(conv, repo, ops, fakeBudget{allow: true})

	_ = ev.Evaluate(context.Background(), task("message.created", "cv1", map[string]any{"id": "m1"}))
	if len(logs.created) != 1 || logs.created[0].Status != entity.EvalSkippedMissingRef {
		t.Errorf("a missing referenced entity must log skipped_missing_ref, got %+v", logs.created)
	}
}

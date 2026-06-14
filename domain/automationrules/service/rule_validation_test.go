package service

import (
	"context"
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/automationrules/entity"
)

type fakeRefChecker struct{ exists map[string]bool }

func (f fakeRefChecker) Exists(_ context.Context, kind, id string) (bool, error) {
	return f.exists[kind+":"+id], nil
}

func ruleSvcWithRefs(refs RefChecker) *RuleService {
	svc, _ := newRuleSvc("wh1")
	svc.SetRefChecker(refs)
	return svc
}

func action(t entity.ActionType, params map[string]string) []entity.Action {
	return []entity.Action{{Type: t, Params: params}}
}

func TestRuleValidate_AssignAgentUnknownRejected(t *testing.T) {
	svc := ruleSvcWithRefs(fakeRefChecker{exists: map[string]bool{}})
	_, err := svc.Create(ruleCtx(), CreateRule{
		Name: "assign", Event: entity.EventConversationCreated,
		Actions: action(entity.ActionAssignAgent, map[string]string{"agent_id": "ghost"}),
	})
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("expected validation for unknown agent, got %v", err)
	}
}

func TestRuleValidate_AssignAgentKnownAccepted(t *testing.T) {
	svc := ruleSvcWithRefs(fakeRefChecker{exists: map[string]bool{"agent:a1": true}})
	if _, err := svc.Create(ruleCtx(), CreateRule{
		Name: "assign", Event: entity.EventConversationCreated,
		Actions: action(entity.ActionAssignAgent, map[string]string{"agent_id": "a1"}),
	}); err != nil {
		t.Fatalf("known agent must be accepted: %v", err)
	}
}

func TestRuleValidate_ChangePriorityInvalidRejected(t *testing.T) {
	svc, _ := newRuleSvc("wh1")
	_, err := svc.Create(ruleCtx(), CreateRule{
		Name: "prio", Event: entity.EventConversationCreated,
		Actions: action(entity.ActionChangePriority, map[string]string{"priority": "sometimes"}),
	})
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("expected validation for invalid priority, got %v", err)
	}
}

func TestRuleValidate_MessageContentConditionAccepted(t *testing.T) {
	svc, _ := newRuleSvc("wh1")
	if _, err := svc.Create(ruleCtx(), CreateRule{
		Name: "content", Event: entity.EventMessageCreated,
		Conditions: []entity.Condition{{Field: entity.FieldMessageContent, Operator: entity.OpContains, Value: "suporte"}},
		Actions:    sendWebhook("wh1"),
	}); err != nil {
		t.Fatalf("message_content/contains must be accepted: %v", err)
	}
}

func TestRuleValidate_NoParamActionsAccepted(t *testing.T) {
	svc, _ := newRuleSvc("wh1")
	if _, err := svc.Create(ruleCtx(), CreateRule{
		Name: "resolve", Event: entity.EventMessageCreated,
		Actions: action(entity.ActionResolveConversation, nil),
	}); err != nil {
		t.Fatalf("resolve_conversation needs no params: %v", err)
	}
}

func TestRuleMissingRefs_FlagsDeletedAgent(t *testing.T) {
	svc := ruleSvcWithRefs(fakeRefChecker{exists: map[string]bool{}}) // nothing exists
	rule := &entity.AutomationRule{
		Actions: action(entity.ActionAssignAgent, map[string]string{"agent_id": "gone"}),
	}
	missing := svc.MissingRefs(ruleCtx(), rule)
	if len(missing) != 1 || missing[0].Kind != "agent" || missing[0].ID != "gone" {
		t.Fatalf("expected one missing agent ref, got %+v", missing)
	}
}

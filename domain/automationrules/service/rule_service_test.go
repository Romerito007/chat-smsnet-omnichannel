package service

import (
	"context"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/automationrules/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	whentity "github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/entity"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

// fakeRuleRepo is an in-memory RuleRepository.
type fakeRuleRepo struct {
	byID map[string]*entity.AutomationRule
}

func newFakeRuleRepo() *fakeRuleRepo { return &fakeRuleRepo{byID: map[string]*entity.AutomationRule{}} }

func (r *fakeRuleRepo) Create(_ context.Context, rule *entity.AutomationRule) error {
	r.byID[rule.ID] = rule
	return nil
}
func (r *fakeRuleRepo) Update(_ context.Context, rule *entity.AutomationRule) error {
	r.byID[rule.ID] = rule
	return nil
}
func (r *fakeRuleRepo) Delete(_ context.Context, id string) error { delete(r.byID, id); return nil }
func (r *fakeRuleRepo) FindByID(_ context.Context, id string) (*entity.AutomationRule, error) {
	if rule, ok := r.byID[id]; ok {
		return rule, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeRuleRepo) List(context.Context) ([]*entity.AutomationRule, error) {
	out := make([]*entity.AutomationRule, 0, len(r.byID))
	for _, rule := range r.byID {
		out = append(out, rule)
	}
	return out, nil
}
func (r *fakeRuleRepo) ListEnabledByEvent(_ context.Context, e entity.RuleEvent) ([]*entity.AutomationRule, error) {
	var out []*entity.AutomationRule
	for _, rule := range r.byID {
		if rule.Enabled && rule.Event == e {
			out = append(out, rule)
		}
	}
	return out, nil
}
func (r *fakeRuleRepo) FindOneByWebhook(_ context.Context, webhookID string) (*entity.AutomationRule, error) {
	for _, rule := range r.byID {
		for _, id := range rule.WebhookIDs() {
			if id == webhookID {
				return rule, nil
			}
		}
	}
	return nil, nil
}

// fakeWebhookSubs implements webhooks repository.SubscriptionRepository (FindByID).
type fakeWebhookSubs struct{ ids map[string]bool }

func (r *fakeWebhookSubs) Create(context.Context, *whentity.WebhookSubscription) error { return nil }
func (r *fakeWebhookSubs) Update(context.Context, *whentity.WebhookSubscription) error { return nil }
func (r *fakeWebhookSubs) Delete(context.Context, string) error                        { return nil }
func (r *fakeWebhookSubs) FindByID(_ context.Context, id string) (*whentity.WebhookSubscription, error) {
	if r.ids[id] {
		return &whentity.WebhookSubscription{ID: id, TenantID: "t1", Enabled: true}, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeWebhookSubs) List(context.Context, shared.PageRequest) ([]*whentity.WebhookSubscription, error) {
	return nil, nil
}
func (r *fakeWebhookSubs) ListEnabledByEvent(context.Context, string, string) ([]*whentity.WebhookSubscription, error) {
	return nil, nil
}

func ruleCtx() context.Context { return shared.WithTenant(context.Background(), "t1") }

func newRuleSvc(webhookIDs ...string) (*RuleService, *fakeRuleRepo) {
	repo := newFakeRuleRepo()
	ids := map[string]bool{}
	for _, id := range webhookIDs {
		ids[id] = true
	}
	svc := NewRuleService(repo, &fakeWebhookSubs{ids: ids}, fixedClock{t: time.Unix(1700000000, 0).UTC()})
	return svc, repo
}

func sendWebhook(id string) []entity.Action {
	return []entity.Action{{Type: entity.ActionSendWebhook, WebhookID: id}}
}

func TestRuleCreate_Valid(t *testing.T) {
	svc, _ := newRuleSvc("wh1")
	r, err := svc.Create(ruleCtx(), CreateRule{
		Name:  "Boas-vindas",
		Event: entity.EventConversationCreated,
		Conditions: []entity.Condition{
			{Field: entity.FieldStatus, Operator: entity.OpEqualTo, Value: "new"},
			{Field: entity.FieldTags, Operator: entity.OpContains, Value: "vip"},
			{Field: entity.FieldContactPhone, Operator: entity.OpContains, Value: "+55"},
		},
		Actions: sendWebhook("wh1"),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !r.Enabled {
		t.Errorf("enabled should default true")
	}
}

func TestRuleCreate_EmptyConditionsAllowed(t *testing.T) {
	svc, _ := newRuleSvc("wh1")
	if _, err := svc.Create(ruleCtx(), CreateRule{
		Name: "Sempre", Event: entity.EventMessageCreated, Actions: sendWebhook("wh1"),
	}); err != nil {
		t.Fatalf("empty conditions must be allowed (match-all): %v", err)
	}
}

func TestRuleCreate_RejectsUnknownEvent(t *testing.T) {
	svc, _ := newRuleSvc("wh1")
	_, err := svc.Create(ruleCtx(), CreateRule{Name: "X", Event: "nope", Actions: sendWebhook("wh1")})
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("expected validation for unknown event, got %v", err)
	}
}

func TestRuleCreate_RejectsBadOperatorForField(t *testing.T) {
	svc, _ := newRuleSvc("wh1")
	// "contains" is not valid for the scalar status field.
	_, err := svc.Create(ruleCtx(), CreateRule{
		Name: "X", Event: entity.EventConversationCreated,
		Conditions: []entity.Condition{{Field: entity.FieldStatus, Operator: entity.OpContains, Value: "new"}},
		Actions:    sendWebhook("wh1"),
	})
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("expected validation for bad operator, got %v", err)
	}
}

func TestRuleCreate_RequiresAction(t *testing.T) {
	svc, _ := newRuleSvc("wh1")
	_, err := svc.Create(ruleCtx(), CreateRule{Name: "X", Event: entity.EventConversationCreated})
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("expected validation for missing action, got %v", err)
	}
}

func TestRuleCreate_RejectsUnknownWebhook(t *testing.T) {
	svc, _ := newRuleSvc() // no webhooks registered
	_, err := svc.Create(ruleCtx(), CreateRule{
		Name: "X", Event: entity.EventConversationCreated, Actions: sendWebhook("ghost"),
	})
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("expected validation for unknown webhook_id, got %v", err)
	}
}

func TestRuleIsWebhookInUse(t *testing.T) {
	svc, _ := newRuleSvc("wh1")
	r, _ := svc.Create(ruleCtx(), CreateRule{Name: "Regra X", Event: entity.EventConversationCreated, Actions: sendWebhook("wh1")})

	inUse, usedBy, err := svc.IsWebhookInUse(ruleCtx(), "wh1")
	if err != nil {
		t.Fatalf("in use: %v", err)
	}
	if !inUse || usedBy != "Regra X" {
		t.Errorf("expected in-use by %q, got inUse=%v usedBy=%q", r.Name, inUse, usedBy)
	}
	if inUse, _, _ := svc.IsWebhookInUse(ruleCtx(), "other"); inUse {
		t.Errorf("unreferenced webhook must not be reported in use")
	}
}

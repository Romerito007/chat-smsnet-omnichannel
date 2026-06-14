// Package service holds the automation-rules business logic: CRUD + validation,
// and the webhook-usage check that blocks deleting a webhook a rule references.
// The async evaluation lives alongside (added with the evaluator).
package service

import (
	"context"
	"strconv"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/automationrules/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/automationrules/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	whrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/repository"
)

// CreateRule is the input to RuleService.Create.
type CreateRule struct {
	Name        string
	Description string
	Event       entity.RuleEvent
	Enabled     *bool // nil → true
	Priority    int
	Conditions  []entity.Condition
	Actions     []entity.Action
}

// UpdateRule carries optional fields; nil pointers mean "leave unchanged".
type UpdateRule struct {
	Name        *string
	Description *string
	Event       *entity.RuleEvent
	Enabled     *bool
	Priority    *int
	Conditions  *[]entity.Condition
	Actions     *[]entity.Action
}

// RefChecker reports whether an action's referenced tenant entity exists/is usable
// (kind ∈ agent|sector|tag|attachment). Used for create-time validation and the
// rule health indicator. Optional: when unset, only structural validation runs.
type RefChecker interface {
	Exists(ctx context.Context, kind, id string) (bool, error)
}

// MissingRef is one action referencing a tenant entity that no longer exists (soft
// referential integrity surfaced as a rule health indicator).
type MissingRef struct {
	ActionIndex int
	Kind        string
	ID          string
}

// RuleService manages automation rules, validates referenced entities, and reads
// evaluation logs.
type RuleService struct {
	repo     repository.RuleRepository
	webhooks whrepo.SubscriptionRepository
	logs     repository.LogRepository
	refs     RefChecker
	clock    shared.Clock
}

// NewRuleService builds the service.
func NewRuleService(repo repository.RuleRepository, webhooks whrepo.SubscriptionRepository, logs repository.LogRepository, clock shared.Clock) *RuleService {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &RuleService{repo: repo, webhooks: webhooks, logs: logs, clock: clock}
}

// SetRefChecker wires the referenced-entity existence checker (agent/sector/tag/
// attachment). Optional: when unset, create-time validation is structural only and
// the health indicator reports no missing refs.
func (s *RuleService) SetRefChecker(r RefChecker) {
	if r != nil {
		s.refs = r
	}
}

// MissingRefs returns the rule's actions whose referenced entity no longer exists.
// Best-effort: a checker error is treated as "exists" so a transient outage never
// flags a healthy rule. Empty result means the rule is healthy.
func (s *RuleService) MissingRefs(ctx context.Context, r *entity.AutomationRule) []MissingRef {
	var out []MissingRef
	for i, a := range r.Actions {
		key, kind, ok := a.Type.ParamKey()
		if !ok {
			continue
		}
		id := strings.TrimSpace(a.Param(key))
		if id == "" {
			continue
		}
		if !s.refExists(ctx, kind, id) {
			out = append(out, MissingRef{ActionIndex: i, Kind: kind, ID: id})
		}
	}
	return out
}

// refExists checks one referenced entity; webhooks go through the subscription
// repo, the rest through the RefChecker. Errors fail "exists" (best-effort).
func (s *RuleService) refExists(ctx context.Context, kind, id string) bool {
	if kind == "webhook" {
		if _, err := s.webhooks.FindByID(ctx, id); err != nil {
			return apperror.From(err).Code != apperror.CodeNotFound
		}
		return true
	}
	if s.refs == nil {
		return true
	}
	ok, err := s.refs.Exists(ctx, kind, id)
	if err != nil {
		return true
	}
	return ok
}

// Logs returns a rule's evaluation logs (GET /v1/automation-rules/{id}/logs),
// after verifying the rule belongs to the tenant.
func (s *RuleService) Logs(ctx context.Context, ruleID string, page shared.PageRequest) ([]*entity.RuleEvaluationLog, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	if _, err := s.repo.FindByID(ctx, ruleID); err != nil {
		return nil, err
	}
	return s.logs.ListByRule(ctx, ruleID, page)
}

// List returns the tenant's rules.
func (s *RuleService) List(ctx context.Context) ([]*entity.AutomationRule, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.List(ctx)
}

// Get returns one rule by id.
func (s *RuleService) Get(ctx context.Context, id string) (*entity.AutomationRule, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}

// Create registers a new rule after validating the event, conditions and actions
// (including that every referenced webhook exists for the tenant).
func (s *RuleService) Create(ctx context.Context, cmd CreateRule) (*entity.AutomationRule, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	enabled := true
	if cmd.Enabled != nil {
		enabled = *cmd.Enabled
	}
	now := s.clock.Now()
	r := &entity.AutomationRule{
		ID:          shared.NewID(),
		TenantID:    tenantID,
		Name:        strings.TrimSpace(cmd.Name),
		Description: strings.TrimSpace(cmd.Description),
		Event:       cmd.Event,
		Enabled:     enabled,
		Priority:    cmd.Priority,
		Conditions:  cmd.Conditions,
		Actions:     cmd.Actions,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.validate(ctx, r); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, r); err != nil {
		return nil, err
	}
	return r, nil
}

// Update applies the non-nil fields and re-validates the rule.
func (s *RuleService) Update(ctx context.Context, id string, cmd UpdateRule) (*entity.AutomationRule, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	r, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if cmd.Name != nil {
		r.Name = strings.TrimSpace(*cmd.Name)
	}
	if cmd.Description != nil {
		r.Description = strings.TrimSpace(*cmd.Description)
	}
	if cmd.Event != nil {
		r.Event = *cmd.Event
	}
	if cmd.Enabled != nil {
		r.Enabled = *cmd.Enabled
	}
	if cmd.Priority != nil {
		r.Priority = *cmd.Priority
	}
	if cmd.Conditions != nil {
		r.Conditions = *cmd.Conditions
	}
	if cmd.Actions != nil {
		r.Actions = *cmd.Actions
	}
	if err := s.validate(ctx, r); err != nil {
		return nil, err
	}
	r.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, r); err != nil {
		return nil, err
	}
	return r, nil
}

// Delete removes a rule.
func (s *RuleService) Delete(ctx context.Context, id string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// IsWebhookInUse reports whether any rule references the webhook, and the name of
// one such rule (for a clear "in use" message). Implements the webhooks
// WebhookUsageChecker port so a webhook delete can be blocked.
func (s *RuleService) IsWebhookInUse(ctx context.Context, webhookID string) (bool, string, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return false, "", err
	}
	r, err := s.repo.FindOneByWebhook(ctx, webhookID)
	if err != nil {
		return false, "", err
	}
	if r == nil {
		return false, "", nil
	}
	return true, r.Name, nil
}

// validate enforces the rule invariants. Conditions may be empty (match-all);
// actions require at least one send_webhook with an existing webhook.
func (s *RuleService) validate(ctx context.Context, r *entity.AutomationRule) error {
	v := map[string]any{}
	if r.Name == "" {
		v["name"] = "is required"
	}
	if !entity.ValidEvent(r.Event) {
		v["event"] = "unknown event"
	}
	for i, c := range r.Conditions {
		if entity.OperatorsFor(c.Field) == nil {
			v[fieldKey("conditions", i, "field")] = "unknown field"
			continue
		}
		if !entity.OperatorAllowed(c.Field, c.Operator) {
			v[fieldKey("conditions", i, "operator")] = "operator not valid for this field"
		}
		if strings.TrimSpace(c.Value) == "" {
			v[fieldKey("conditions", i, "value")] = "is required"
		}
	}
	if len(r.Actions) == 0 {
		v["actions"] = "at least one action is required"
	}
	for i, a := range r.Actions {
		switch a.Type {
		case entity.ActionSendWebhook:
			webhookID := strings.TrimSpace(a.Param("webhook_id"))
			if webhookID == "" {
				v[fieldKey("actions", i, "webhook_id")] = "is required"
				continue
			}
			if _, err := s.webhooks.FindByID(ctx, webhookID); err != nil {
				if apperror.From(err).Code == apperror.CodeNotFound {
					v[fieldKey("actions", i, "webhook_id")] = "unknown webhook_id"
					continue
				}
				return err
			}
		case entity.ActionSendMessage:
			if strings.TrimSpace(a.Param("text")) == "" {
				v[fieldKey("actions", i, "text")] = "is required"
			}
		case entity.ActionSendAttachment:
			if err := s.validateRef(ctx, v, i, "attachment_id", "attachment", a); err != nil {
				return err
			}
		case entity.ActionAssignAgent:
			if err := s.validateRef(ctx, v, i, "agent_id", "agent", a); err != nil {
				return err
			}
		case entity.ActionAssignTeam:
			if err := s.validateRef(ctx, v, i, "sector_id", "sector", a); err != nil {
				return err
			}
		case entity.ActionAddTag, entity.ActionRemoveTag:
			if err := s.validateRef(ctx, v, i, "tag_id", "tag", a); err != nil {
				return err
			}
		case entity.ActionChangePriority:
			if !validPriority(strings.TrimSpace(a.Param("priority"))) {
				v[fieldKey("actions", i, "priority")] = "must be one of: low, normal, high, urgent"
			}
		case entity.ActionRemoveAssignedAgent, entity.ActionRemoveAssignedTeam,
			entity.ActionResolveConversation, entity.ActionOpenConversation, entity.ActionMarkPending:
			// no params
		default:
			v[fieldKey("actions", i, "type")] = "unsupported action type"
		}
	}
	if len(v) > 0 {
		return apperror.Validation("invalid automation rule").WithDetails(v)
	}
	return nil
}

func fieldKey(group string, i int, sub string) string {
	return group + "[" + strconv.Itoa(i) + "]." + sub
}

// validateRef checks an action's required param is present and (when a RefChecker
// is wired) references an existing tenant entity. A backend error is returned;
// missing/unknown is recorded in v.
func (s *RuleService) validateRef(ctx context.Context, v map[string]any, i int, paramKey, kind string, a entity.Action) error {
	id := strings.TrimSpace(a.Param(paramKey))
	if id == "" {
		v[fieldKey("actions", i, paramKey)] = "is required"
		return nil
	}
	if s.refs == nil {
		return nil // structural validation only
	}
	ok, err := s.refs.Exists(ctx, kind, id)
	if err != nil {
		return err
	}
	if !ok {
		v[fieldKey("actions", i, paramKey)] = "unknown " + paramKey
	}
	return nil
}

// validPriority reports whether p is a known conversation priority.
func validPriority(p string) bool {
	switch p {
	case "low", "normal", "high", "urgent":
		return true
	}
	return false
}

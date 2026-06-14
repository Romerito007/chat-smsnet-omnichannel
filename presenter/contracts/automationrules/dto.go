// Package automationrules holds the request/response DTOs for the automation-rules
// endpoints.
package automationrules

import (
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/automationrules/entity"
	arservice "github.com/romerito007/chat-smsnet-omnichannel/domain/automationrules/service"
)

// ConditionDTO is one field/operator/value test.
type ConditionDTO struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

// ActionDTO is one action. Each action reads only its own params: send_webhook →
// webhook_id; send_message → text; send_attachment → attachment_id; assign_agent →
// agent_id; assign_team → sector_id; add_tag/remove_tag → tag_id; change_priority
// → priority. remove_*/resolve/open/mark_pending take no params.
type ActionDTO struct {
	Type         string `json:"type"`
	WebhookID    string `json:"webhook_id,omitempty"`
	Text         string `json:"text,omitempty"`
	AttachmentID string `json:"attachment_id,omitempty"`
	AgentID      string `json:"agent_id,omitempty"`
	SectorID     string `json:"sector_id,omitempty"`
	TagID        string `json:"tag_id,omitempty"`
	Priority     string `json:"priority,omitempty"`
}

// MissingRefDTO is one action whose referenced entity no longer exists.
type MissingRefDTO struct {
	ActionIndex int    `json:"action_index"`
	Kind        string `json:"kind"`
	ID          string `json:"id"`
}

// RuleHealthDTO surfaces referential health: ok=false lists the missing refs so the
// UI can flag "references a deleted agent/tag/sector/webhook".
type RuleHealthDTO struct {
	OK          bool            `json:"ok"`
	MissingRefs []MissingRefDTO `json:"missing_refs,omitempty"`
}

// RuleResponse is the public representation of an automation rule.
type RuleResponse struct {
	ID          string         `json:"id"`
	TenantID    string         `json:"tenant_id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Event       string         `json:"event"`
	Enabled     bool           `json:"enabled"`
	Priority    int            `json:"priority"`
	Conditions  []ConditionDTO `json:"conditions"`
	Actions     []ActionDTO    `json:"actions"`
	Health      *RuleHealthDTO `json:"health,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// NewRuleResponse maps a rule entity to the DTO (without the health indicator).
func NewRuleResponse(r *entity.AutomationRule) RuleResponse {
	conds := make([]ConditionDTO, 0, len(r.Conditions))
	for _, c := range r.Conditions {
		conds = append(conds, ConditionDTO{Field: string(c.Field), Operator: string(c.Operator), Value: c.Value})
	}
	acts := make([]ActionDTO, 0, len(r.Actions))
	for _, a := range r.Actions {
		acts = append(acts, ActionDTO{
			Type:         string(a.Type),
			WebhookID:    a.Param("webhook_id"),
			Text:         a.Param("text"),
			AttachmentID: a.Param("attachment_id"),
			AgentID:      a.Param("agent_id"),
			SectorID:     a.Param("sector_id"),
			TagID:        a.Param("tag_id"),
			Priority:     a.Param("priority"),
		})
	}
	return RuleResponse{
		ID:          r.ID,
		TenantID:    r.TenantID,
		Name:        r.Name,
		Description: r.Description,
		Event:       string(r.Event),
		Enabled:     r.Enabled,
		Priority:    r.Priority,
		Conditions:  conds,
		Actions:     acts,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

// NewRuleResponseWithHealth maps a rule and attaches its referential health.
func NewRuleResponseWithHealth(r *entity.AutomationRule, missing []arservice.MissingRef) RuleResponse {
	resp := NewRuleResponse(r)
	health := &RuleHealthDTO{OK: len(missing) == 0}
	for _, m := range missing {
		health.MissingRefs = append(health.MissingRefs, MissingRefDTO{ActionIndex: m.ActionIndex, Kind: m.Kind, ID: m.ID})
	}
	resp.Health = health
	return resp
}

// NewRuleListResponse wraps rules in a { data: [...] } envelope.
func NewRuleListResponse(rs []*entity.AutomationRule) map[string]any {
	out := make([]RuleResponse, 0, len(rs))
	for _, r := range rs {
		out = append(out, NewRuleResponse(r))
	}
	return map[string]any{"data": out}
}

// NewRuleListResponseWithHealth wraps rules with a per-rule health indicator.
func NewRuleListResponseWithHealth(rs []*entity.AutomationRule, healthFor func(*entity.AutomationRule) []arservice.MissingRef) map[string]any {
	out := make([]RuleResponse, 0, len(rs))
	for _, r := range rs {
		out = append(out, NewRuleResponseWithHealth(r, healthFor(r)))
	}
	return map[string]any{"data": out}
}

// CreateRuleRequest is the body of POST /v1/automation-rules.
type CreateRuleRequest struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Event       string         `json:"event"`
	Enabled     *bool          `json:"enabled"`
	Priority    int            `json:"priority"`
	Conditions  []ConditionDTO `json:"conditions"`
	Actions     []ActionDTO    `json:"actions"`
}

// ToCommand maps to the service command.
func (r CreateRuleRequest) ToCommand() arservice.CreateRule {
	return arservice.CreateRule{
		Name:        r.Name,
		Description: r.Description,
		Event:       entity.RuleEvent(r.Event),
		Enabled:     r.Enabled,
		Priority:    r.Priority,
		Conditions:  toConditions(r.Conditions),
		Actions:     toActions(r.Actions),
	}
}

// UpdateRuleRequest is the body of PATCH /v1/automation-rules/{id}.
type UpdateRuleRequest struct {
	Name        *string         `json:"name"`
	Description *string         `json:"description"`
	Event       *string         `json:"event"`
	Enabled     *bool           `json:"enabled"`
	Priority    *int            `json:"priority"`
	Conditions  *[]ConditionDTO `json:"conditions"`
	Actions     *[]ActionDTO    `json:"actions"`
}

// ToCommand maps to the service command.
func (r UpdateRuleRequest) ToCommand() arservice.UpdateRule {
	cmd := arservice.UpdateRule{Name: r.Name, Description: r.Description, Enabled: r.Enabled, Priority: r.Priority}
	if r.Event != nil {
		e := entity.RuleEvent(*r.Event)
		cmd.Event = &e
	}
	if r.Conditions != nil {
		c := toConditions(*r.Conditions)
		cmd.Conditions = &c
	}
	if r.Actions != nil {
		a := toActions(*r.Actions)
		cmd.Actions = &a
	}
	return cmd
}

func toConditions(in []ConditionDTO) []entity.Condition {
	out := make([]entity.Condition, 0, len(in))
	for _, c := range in {
		out = append(out, entity.Condition{
			Field:    entity.ConditionField(c.Field),
			Operator: entity.ConditionOperator(c.Operator),
			Value:    c.Value,
		})
	}
	return out
}

func toActions(in []ActionDTO) []entity.Action {
	out := make([]entity.Action, 0, len(in))
	for _, a := range in {
		params := map[string]string{}
		setParam(params, "webhook_id", a.WebhookID)
		setParam(params, "text", a.Text)
		setParam(params, "attachment_id", a.AttachmentID)
		setParam(params, "agent_id", a.AgentID)
		setParam(params, "sector_id", a.SectorID)
		setParam(params, "tag_id", a.TagID)
		setParam(params, "priority", a.Priority)
		out = append(out, entity.Action{Type: entity.ActionType(a.Type), Params: params})
	}
	return out
}

func setParam(m map[string]string, key, val string) {
	if val != "" {
		m[key] = val
	}
}

// EvaluationLogResponse is one rule-firing log entry.
type EvaluationLogResponse struct {
	ID             string    `json:"id"`
	RuleID         string    `json:"rule_id"`
	Event          string    `json:"event"`
	ConversationID string    `json:"conversation_id,omitempty"`
	Status         string    `json:"status"`
	ErrorSummary   string    `json:"error_summary,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// NewEvaluationLogResponses maps a slice of log entities to DTOs.
func NewEvaluationLogResponses(ls []*entity.RuleEvaluationLog) []EvaluationLogResponse {
	out := make([]EvaluationLogResponse, 0, len(ls))
	for _, l := range ls {
		out = append(out, NewEvaluationLogResponse(l))
	}
	return out
}

// NewEvaluationLogResponse maps a log entity to the DTO.
func NewEvaluationLogResponse(l *entity.RuleEvaluationLog) EvaluationLogResponse {
	return EvaluationLogResponse{
		ID:             l.ID,
		RuleID:         l.RuleID,
		Event:          string(l.Event),
		ConversationID: l.ConversationID,
		Status:         string(l.Status),
		ErrorSummary:   l.ErrorSummary,
		CreatedAt:      l.CreatedAt,
	}
}

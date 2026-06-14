// Package entity holds the automation-rules aggregate: a tenant-defined rule that
// reacts to a conversation/message lifecycle event, matches conditions against the
// conversation/contact, and runs actions (currently only send_webhook). This is a
// DISTINCT concept from the `automation` domain (external flow orchestration).
package entity

import "time"

// RuleEvent is the lifecycle event a rule reacts to (front-facing wire names).
type RuleEvent string

const (
	EventConversationCreated  RuleEvent = "conversation_created"
	EventConversationUpdated  RuleEvent = "conversation_updated"
	EventConversationResolved RuleEvent = "conversation_resolved"
	EventConversationOpened   RuleEvent = "conversation_opened"
	EventConversationClosed   RuleEvent = "conversation_closed"
	EventMessageCreated       RuleEvent = "message_created"
)

// internalToRuleEvent maps the internal dot-notation events emitted by the
// conversations service to the rule wire events. Only these internal events
// trigger rule evaluation; everything else is ignored by the sink.
var internalToRuleEvent = map[string]RuleEvent{
	"conversation.created":  EventConversationCreated,
	"conversation.updated":  EventConversationUpdated,
	"conversation.resolved": EventConversationResolved,
	"conversation.reopened": EventConversationOpened,
	"conversation.closed":   EventConversationClosed,
	"message.created":       EventMessageCreated,
}

// RuleEventForInternal maps an internal event name to its rule event; ok is false
// when the internal event is not one rules react to.
func RuleEventForInternal(internal string) (RuleEvent, bool) {
	e, ok := internalToRuleEvent[internal]
	return e, ok
}

// ValidEvent reports whether e is a known rule event.
func ValidEvent(e RuleEvent) bool {
	for _, v := range AllEvents {
		if v == e {
			return true
		}
	}
	return false
}

// AllEvents lists every supported rule event.
var AllEvents = []RuleEvent{
	EventConversationCreated, EventConversationUpdated, EventConversationResolved,
	EventConversationOpened, EventConversationClosed, EventMessageCreated,
}

// ConditionField is a conversation/contact field a condition tests.
type ConditionField string

const (
	FieldStatus          ConditionField = "status"
	FieldChannel         ConditionField = "channel"
	FieldAssignedAgentID ConditionField = "assigned_agent_id"
	FieldSectorID        ConditionField = "sector_id"
	FieldQueueID         ConditionField = "queue_id"
	FieldPriority        ConditionField = "priority"
	FieldTags            ConditionField = "tags"
	FieldContactPhone    ConditionField = "contact_phone"
)

// ConditionOperator is how a field is compared to the condition value.
type ConditionOperator string

const (
	OpEqualTo        ConditionOperator = "equal_to"
	OpNotEqualTo     ConditionOperator = "not_equal_to"
	OpContains       ConditionOperator = "contains"
	OpDoesNotContain ConditionOperator = "does_not_contain"
)

// allowedOperators is the closed set of operators valid per field, by field type:
// scalar string fields → equal/not_equal; the tags list → contains/does_not_contain;
// phone (string) → equal/contains.
var allowedOperators = map[ConditionField][]ConditionOperator{
	FieldStatus:          {OpEqualTo, OpNotEqualTo},
	FieldChannel:         {OpEqualTo, OpNotEqualTo},
	FieldAssignedAgentID: {OpEqualTo, OpNotEqualTo},
	FieldSectorID:        {OpEqualTo, OpNotEqualTo},
	FieldQueueID:         {OpEqualTo, OpNotEqualTo},
	FieldPriority:        {OpEqualTo, OpNotEqualTo},
	FieldTags:            {OpContains, OpDoesNotContain},
	FieldContactPhone:    {OpEqualTo, OpContains},
}

// OperatorsFor returns the operators valid for a field (nil for unknown fields).
func OperatorsFor(field ConditionField) []ConditionOperator {
	return allowedOperators[field]
}

// OperatorAllowed reports whether op is valid for field.
func OperatorAllowed(field ConditionField, op ConditionOperator) bool {
	for _, o := range allowedOperators[field] {
		if o == op {
			return true
		}
	}
	return false
}

// ActionType is the kind of action a rule runs.
type ActionType string

const (
	// ActionSendWebhook delivers the event to an existing webhook (by id) via the
	// webhooks pipeline. Param: webhook_id.
	ActionSendWebhook ActionType = "send_webhook"
	// ActionSendMessage injects an outbound message authored by automation
	// (SenderType=automation), reusing the normal send pipeline. Param: text.
	ActionSendMessage ActionType = "send_message"
)

// Condition is one field/operator/value test. Value is a single string (a tag id
// for the tags field); multiple conditions on a rule combine with AND.
type Condition struct {
	Field    ConditionField
	Operator ConditionOperator
	Value    string
}

// Action is one thing a rule does on match. Params holds the action's typed
// inputs by key (e.g. "webhook_id", "text"), kept as an open map so new action
// types add params without changing the schema.
type Action struct {
	Type   ActionType
	Params map[string]string
}

// Param returns the named param value (empty when unset).
func (a Action) Param(key string) string {
	if a.Params == nil {
		return ""
	}
	return a.Params[key]
}

// AutomationRule is a tenant's automation rule: event + AND-conditions + actions.
type AutomationRule struct {
	ID          string
	TenantID    string
	Name        string
	Description string
	Event       RuleEvent
	Enabled     bool
	Conditions  []Condition // combined with AND; empty = match every occurrence
	Actions     []Action
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// WebhookIDs returns the webhook ids referenced by the rule's send_webhook actions.
func (r *AutomationRule) WebhookIDs() []string {
	var out []string
	for _, a := range r.Actions {
		if a.Type == ActionSendWebhook && a.Param("webhook_id") != "" {
			out = append(out, a.Param("webhook_id"))
		}
	}
	return out
}

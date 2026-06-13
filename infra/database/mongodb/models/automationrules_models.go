package models

import "time"

// AutomationRule is the BSON document for an automation rule.
type AutomationRule struct {
	Base        `bson:",inline"`
	Name        string          `bson:"name"`
	Description string          `bson:"description,omitempty"`
	Event       string          `bson:"event"`
	Enabled     bool            `bson:"enabled"`
	Conditions  []RuleCondition `bson:"conditions,omitempty"`
	Actions     []RuleAction    `bson:"actions,omitempty"`
}

// RuleCondition is one field/operator/value test.
type RuleCondition struct {
	Field    string `bson:"field"`
	Operator string `bson:"operator"`
	Value    string `bson:"value"`
}

// RuleAction is one action; for send_webhook, WebhookID references a webhook.
type RuleAction struct {
	Type      string `bson:"type"`
	WebhookID string `bson:"webhook_id,omitempty"`
}

// RuleEvaluationLog is the BSON document for one rule firing (no event payload).
type RuleEvaluationLog struct {
	ID             string    `bson:"_id"`
	TenantID       string    `bson:"tenant_id"`
	RuleID         string    `bson:"rule_id"`
	Event          string    `bson:"event"`
	ConversationID string    `bson:"conversation_id,omitempty"`
	Status         string    `bson:"status"`
	ErrorSummary   string    `bson:"error_summary,omitempty"`
	CreatedAt      time.Time `bson:"created_at"`
}

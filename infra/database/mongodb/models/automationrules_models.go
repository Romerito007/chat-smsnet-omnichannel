package models

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

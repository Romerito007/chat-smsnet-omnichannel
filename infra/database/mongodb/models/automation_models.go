package models

import "time"

// AutomationIntegration is the BSON document for an automation integration. The
// secret is stored encrypted.
type AutomationIntegration struct {
	Base            `bson:",inline"`
	Name            string `bson:"name,omitempty"`
	BaseURL         string `bson:"base_url"`
	AuthType        string `bson:"auth_type,omitempty"`
	EncryptedSecret string `bson:"encrypted_secret,omitempty"`
	Enabled         bool   `bson:"enabled"`
	TimeoutMs       int    `bson:"timeout_ms"`
}

// AutomationRun is the BSON document for one automation execution.
type AutomationRun struct {
	ID             string         `bson:"_id"`
	TenantID       string         `bson:"tenant_id"`
	ConversationID string         `bson:"conversation_id"`
	MessageID      string         `bson:"message_id,omitempty"`
	ExternalRunID  string         `bson:"external_run_id,omitempty"`
	Status         string         `bson:"status"`
	Input          map[string]any `bson:"input,omitempty"`
	Output         map[string]any `bson:"output,omitempty"`
	Error          string         `bson:"error,omitempty"`
	CreatedAt      time.Time      `bson:"created_at"`
	UpdatedAt      time.Time      `bson:"updated_at"`
}

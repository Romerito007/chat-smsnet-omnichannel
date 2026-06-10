package models

import "time"

// ProviderIntegrationConfig is the BSON document for the providerhub config. The
// secret is stored encrypted.
type ProviderIntegrationConfig struct {
	Base            `bson:",inline"`
	Name            string `bson:"name,omitempty"`
	BaseURL         string `bson:"base_url"`
	AuthType        string `bson:"auth_type,omitempty"`
	EncryptedSecret string `bson:"encrypted_secret,omitempty"`
	Enabled         bool   `bson:"enabled"`
	TimeoutMs       int    `bson:"timeout_ms"`
}

// ProviderQueryLog is the BSON document for the minimal technical query log.
// It deliberately stores no response body.
type ProviderQueryLog struct {
	ID             string    `bson:"_id"`
	TenantID       string    `bson:"tenant_id"`
	UserID         string    `bson:"user_id,omitempty"`
	ContactID      string    `bson:"contact_id,omitempty"`
	ConversationID string    `bson:"conversation_id,omitempty"`
	QueryType      string    `bson:"query_type"`
	Status         string    `bson:"status"`
	LatencyMs      int64     `bson:"latency_ms"`
	ErrorSummary   string    `bson:"error_summary,omitempty"`
	CreatedAt      time.Time `bson:"created_at"`
}

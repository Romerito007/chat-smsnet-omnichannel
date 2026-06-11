package models

import "time"

// McpServer is the BSON document for a tenant's MCP server registration. The auth
// token is stored encrypted (encrypted_auth_token); plaintext is never persisted.
type McpServer struct {
	Base               `bson:",inline"`
	Name               string `bson:"name"`
	Transport          string `bson:"transport"`
	BaseURL            string `bson:"base_url"`
	AuthHeader         string `bson:"auth_header,omitempty"`
	EncryptedAuthToken string `bson:"encrypted_auth_token,omitempty"`
	Kind               string `bson:"kind"`
	Enabled            bool   `bson:"enabled"`
}

// McpApproval is the BSON document for a proposed write action. Args are the
// non-secret tool parameters shown on the confirmation card and replayed on
// execution; no provider secrets are stored.
type McpApproval struct {
	ID             string         `bson:"_id"`
	TenantID       string         `bson:"tenant_id"`
	ConversationID string         `bson:"conversation_id"`
	ServerID       string         `bson:"server_id"`
	ServerName     string         `bson:"server_name"`
	Tool           string         `bson:"tool"`
	Args           map[string]any `bson:"args,omitempty"`
	Status         string         `bson:"status"`
	ProposedBy     string         `bson:"proposed_by,omitempty"`
	DecidedBy      string         `bson:"decided_by,omitempty"`
	Reason         string         `bson:"reason,omitempty"`
	Result         string         `bson:"result,omitempty"`
	Error          string         `bson:"error,omitempty"`
	CreatedAt      time.Time      `bson:"created_at"`
	DecidedAt      *time.Time     `bson:"decided_at,omitempty"`
}

// McpCallLog is the BSON document for a minimal, payload-free tool-call record:
// who/where/which tool + outcome + latency, never the arguments or result.
type McpCallLog struct {
	ID             string    `bson:"_id"`
	TenantID       string    `bson:"tenant_id"`
	UserID         string    `bson:"user_id,omitempty"`
	ConversationID string    `bson:"conversation_id,omitempty"`
	ServerID       string    `bson:"server_id"`
	ServerName     string    `bson:"server_name"`
	Tool           string    `bson:"tool"`
	Write          bool      `bson:"write"`
	Status         string    `bson:"status"`
	LatencyMs      int64     `bson:"latency_ms"`
	ErrorSummary   string    `bson:"error_summary,omitempty"`
	CreatedAt      time.Time `bson:"created_at"`
}

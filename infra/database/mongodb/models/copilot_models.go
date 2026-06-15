package models

import "time"

// AIConfig is the BSON document for a tenant's copilot AI infrastructure.
type AIConfig struct {
	Base            `bson:",inline"`
	Provider        string `bson:"provider"`
	Model           string `bson:"model"`
	EncryptedAPIKey string `bson:"encrypted_api_key,omitempty"`
	BaseURL         string `bson:"base_url,omitempty"`
	Enabled         bool   `bson:"enabled"`
}

// Assistant is the BSON document for a copilot assistant (many per tenant). It
// carries the per-assistant behavior (gates, sampling, persona).
type Assistant struct {
	Base                  `bson:",inline"`
	Name                  string   `bson:"name"`
	ChannelIDs            []string `bson:"channel_ids,omitempty"`
	ISPProfileID          string   `bson:"isp_profile_id,omitempty"`
	MCPServerID           string   `bson:"mcp_server_id,omitempty"`
	AllowCustomerData     bool     `bson:"allow_customer_data"`
	HumanApprovalRequired bool     `bson:"human_approval_required"`
	Temperature           float64  `bson:"temperature"`
	MaxTokens             int      `bson:"max_tokens"`
	SystemInstructions    string   `bson:"system_instructions,omitempty"`
	Enabled               bool     `bson:"enabled"`
}

// AILog is the BSON document for one copilot call. It stores only summaries of
// the input and output, never the full prompt or raw customer data.
type AILog struct {
	ID             string    `bson:"_id"`
	TenantID       string    `bson:"tenant_id"`
	UserID         string    `bson:"user_id,omitempty"`
	ConversationID string    `bson:"conversation_id,omitempty"`
	Provider       string    `bson:"provider"`
	Model          string    `bson:"model"`
	Action         string    `bson:"action"`
	InputSummary   string    `bson:"input_summary,omitempty"`
	OutputSummary  string    `bson:"output_summary,omitempty"`
	TokensInput    int       `bson:"tokens_input"`
	TokensOutput   int       `bson:"tokens_output"`
	EstimatedCost  float64   `bson:"estimated_cost"`
	Status         string    `bson:"status"`
	Error          string    `bson:"error,omitempty"`
	CreatedAt      time.Time `bson:"created_at"`
}

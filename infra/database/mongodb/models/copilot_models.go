package models

import "time"

// AIConfig is the BSON document for a tenant's copilot configuration.
type AIConfig struct {
	Base                  `bson:",inline"`
	Provider              string  `bson:"provider"`
	Model                 string  `bson:"model"`
	Temperature           float64 `bson:"temperature"`
	MaxTokens             int     `bson:"max_tokens"`
	AllowCustomerData     bool    `bson:"allow_customer_data"`
	AllowFinancialData    bool    `bson:"allow_financial_data"`
	AllowMonitoringData   bool    `bson:"allow_monitoring_data"`
	HumanApprovalRequired bool    `bson:"human_approval_required"`
	Enabled               bool    `bson:"enabled"`
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

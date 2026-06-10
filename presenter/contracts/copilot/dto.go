// Package copilot holds the request/response DTOs for the copilot endpoints.
package copilot

import (
	"time"

	ccontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/contracts"
	centity "github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
)

// SaveConfigRequest is the body of PATCH /v1/copilot/config (upsert).
type SaveConfigRequest struct {
	Provider              *string  `json:"provider"`
	Model                 *string  `json:"model"`
	Temperature           *float64 `json:"temperature"`
	MaxTokens             *int     `json:"max_tokens"`
	AllowCustomerData     *bool    `json:"allow_customer_data"`
	AllowFinancialData    *bool    `json:"allow_financial_data"`
	AllowMonitoringData   *bool    `json:"allow_monitoring_data"`
	HumanApprovalRequired *bool    `json:"human_approval_required"`
	Enabled               *bool    `json:"enabled"`
}

// ToCommand maps to the service command.
func (r SaveConfigRequest) ToCommand() ccontracts.SaveConfig {
	return ccontracts.SaveConfig{
		Provider:              r.Provider,
		Model:                 r.Model,
		Temperature:           r.Temperature,
		MaxTokens:             r.MaxTokens,
		AllowCustomerData:     r.AllowCustomerData,
		AllowFinancialData:    r.AllowFinancialData,
		AllowMonitoringData:   r.AllowMonitoringData,
		HumanApprovalRequired: r.HumanApprovalRequired,
		Enabled:               r.Enabled,
	}
}

// ConfigResponse is the public representation of a tenant's copilot config.
type ConfigResponse struct {
	ID                    string    `json:"id"`
	TenantID              string    `json:"tenant_id"`
	Provider              string    `json:"provider"`
	Model                 string    `json:"model"`
	Temperature           float64   `json:"temperature"`
	MaxTokens             int       `json:"max_tokens"`
	AllowCustomerData     bool      `json:"allow_customer_data"`
	AllowFinancialData    bool      `json:"allow_financial_data"`
	AllowMonitoringData   bool      `json:"allow_monitoring_data"`
	HumanApprovalRequired bool      `json:"human_approval_required"`
	Enabled               bool      `json:"enabled"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// NewConfigResponse maps a config entity.
func NewConfigResponse(c *centity.AIConfig) ConfigResponse {
	return ConfigResponse{
		ID:                    c.ID,
		TenantID:              c.TenantID,
		Provider:              string(c.Provider),
		Model:                 c.Model,
		Temperature:           c.Temperature,
		MaxTokens:             c.MaxTokens,
		AllowCustomerData:     c.AllowCustomerData,
		AllowFinancialData:    c.AllowFinancialData,
		AllowMonitoringData:   c.AllowMonitoringData,
		HumanApprovalRequired: c.HumanApprovalRequired,
		Enabled:               c.Enabled,
		CreatedAt:             c.CreatedAt,
		UpdatedAt:             c.UpdatedAt,
	}
}

// SuggestReplyRequest is the body of POST /v1/copilot/suggest-reply.
type SuggestReplyRequest struct {
	ConversationID string `json:"conversation_id"`
	Instruction    string `json:"instruction"`
}

// SummarizeRequest is the body of POST /v1/copilot/summarize.
type SummarizeRequest struct {
	ConversationID string `json:"conversation_id"`
}

// ClassifyRequest is the body of POST /v1/copilot/classify.
type ClassifyRequest struct {
	ConversationID string   `json:"conversation_id"`
	Categories     []string `json:"categories"`
}

// NextActionRequest is the body of POST /v1/copilot/next-action.
type NextActionRequest struct {
	ConversationID string `json:"conversation_id"`
}

// Package copilot holds the request/response DTOs for the copilot endpoints.
package copilot

import (
	"time"

	ccontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/contracts"
	centity "github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
)

// SaveConfigRequest is the body of PATCH /v1/copilot/config (upsert). It carries
// only the AI infrastructure; behavior (gates/sampling/persona) is set per
// assistant, not here.
type SaveConfigRequest struct {
	Provider *string `json:"provider"`
	Model    *string `json:"model"`
	APIKey   *string `json:"api_key"`
	BaseURL  *string `json:"base_url"`
	Enabled  *bool   `json:"enabled"`
}

// ToCommand maps to the service command.
func (r SaveConfigRequest) ToCommand() ccontracts.SaveConfig {
	return ccontracts.SaveConfig{
		Provider: r.Provider,
		Model:    r.Model,
		APIKey:   r.APIKey,
		BaseURL:  r.BaseURL,
		Enabled:  r.Enabled,
	}
}

// ConfigResponse is the public representation of a tenant's copilot AI infra. The
// API key is never returned — only whether one is set (HasKey).
type ConfigResponse struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Provider  string    `json:"provider"`
	Model     string    `json:"model"`
	HasKey    bool      `json:"has_key"`
	BaseURL   string    `json:"base_url,omitempty"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewConfigResponse maps a config entity, masking the API key.
func NewConfigResponse(c *centity.AIConfig) ConfigResponse {
	return ConfigResponse{
		ID:        c.ID,
		TenantID:  c.TenantID,
		Provider:  string(c.Provider),
		Model:     c.Model,
		HasKey:    c.APIKey != "",
		BaseURL:   c.BaseURL,
		Enabled:   c.Enabled,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
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

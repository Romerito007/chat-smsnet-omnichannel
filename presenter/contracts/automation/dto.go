// Package automation holds the request/response DTOs for the automation
// endpoints. Secrets are never returned (masked).
package automation

import (
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/automation/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/automation/entity"
)

// CreateIntegrationRequest is the body of POST /v1/automation/integrations.
type CreateIntegrationRequest struct {
	Name      string `json:"name"`
	BaseURL   string `json:"base_url"`
	AuthType  string `json:"auth_type"`
	Secret    string `json:"secret"`
	TimeoutMs int    `json:"timeout_ms"`
}

// ToCommand maps to the service command.
func (r CreateIntegrationRequest) ToCommand() contracts.CreateIntegration {
	return contracts.CreateIntegration{
		Name:      r.Name,
		BaseURL:   r.BaseURL,
		AuthType:  r.AuthType,
		Secret:    r.Secret,
		TimeoutMs: r.TimeoutMs,
	}
}

// UpdateIntegrationRequest is the body of PATCH /v1/automation/integrations/{id}.
type UpdateIntegrationRequest struct {
	Name      *string `json:"name"`
	BaseURL   *string `json:"base_url"`
	AuthType  *string `json:"auth_type"`
	Secret    *string `json:"secret"`
	Enabled   *bool   `json:"enabled"`
	TimeoutMs *int    `json:"timeout_ms"`
}

// ToCommand maps to the service command.
func (r UpdateIntegrationRequest) ToCommand() contracts.UpdateIntegration {
	return contracts.UpdateIntegration{
		Name:      r.Name,
		BaseURL:   r.BaseURL,
		AuthType:  r.AuthType,
		Secret:    r.Secret,
		Enabled:   r.Enabled,
		TimeoutMs: r.TimeoutMs,
	}
}

// IntegrationResponse is the public representation (secret masked).
type IntegrationResponse struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Name      string    `json:"name,omitempty"`
	BaseURL   string    `json:"base_url"`
	AuthType  string    `json:"auth_type,omitempty"`
	HasSecret bool      `json:"has_secret"`
	Enabled   bool      `json:"enabled"`
	TimeoutMs int       `json:"timeout_ms"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewIntegrationResponse maps an integration entity.
func NewIntegrationResponse(i *entity.AutomationIntegration) IntegrationResponse {
	return IntegrationResponse{
		ID:        i.ID,
		TenantID:  i.TenantID,
		Name:      i.Name,
		BaseURL:   i.BaseURL,
		AuthType:  i.AuthType,
		HasSecret: i.Secret != "",
		Enabled:   i.Enabled,
		TimeoutMs: i.TimeoutMs,
		CreatedAt: i.CreatedAt,
		UpdatedAt: i.UpdatedAt,
	}
}

// NewIntegrationResponses maps a slice.
func NewIntegrationResponses(items []*entity.AutomationIntegration) []IntegrationResponse {
	out := make([]IntegrationResponse, len(items))
	for i, it := range items {
		out[i] = NewIntegrationResponse(it)
	}
	return out
}

// RunResponse is the public representation of an automation run.
type RunResponse struct {
	ID             string         `json:"id"`
	TenantID       string         `json:"tenant_id"`
	ConversationID string         `json:"conversation_id"`
	MessageID      string         `json:"message_id,omitempty"`
	ExternalRunID  string         `json:"external_run_id,omitempty"`
	Status         string         `json:"status"`
	Input          map[string]any `json:"input,omitempty"`
	Output         map[string]any `json:"output,omitempty"`
	Error          string         `json:"error,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// NewRunResponse maps a run entity.
func NewRunResponse(r *entity.AutomationRun) RunResponse {
	return RunResponse{
		ID:             r.ID,
		TenantID:       r.TenantID,
		ConversationID: r.ConversationID,
		MessageID:      r.MessageID,
		ExternalRunID:  r.ExternalRunID,
		Status:         string(r.Status),
		Input:          r.Input,
		Output:         r.Output,
		Error:          r.Error,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
}

// NewRunResponses maps a slice.
func NewRunResponses(items []*entity.AutomationRun) []RunResponse {
	out := make([]RunResponse, len(items))
	for i, it := range items {
		out[i] = NewRunResponse(it)
	}
	return out
}

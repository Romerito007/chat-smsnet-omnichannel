// Package providerhub holds the request/response DTOs for the providerhub
// endpoints. The config secret is never returned (masked); external query
// payloads pass through the normalized domain DTOs and are never persisted.
package providerhub

import (
	"time"

	phcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/contracts"
	phentity "github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/entity"
)

// CreateConfigRequest is the body of POST /v1/providerhub/config.
type CreateConfigRequest struct {
	Name      string `json:"name"`
	BaseURL   string `json:"base_url"`
	AuthType  string `json:"auth_type"`
	Secret    string `json:"secret"`
	TimeoutMs int    `json:"timeout_ms"`
}

// ToCommand maps to the service command.
func (r CreateConfigRequest) ToCommand() phcontracts.CreateConfig {
	return phcontracts.CreateConfig{
		Name:      r.Name,
		BaseURL:   r.BaseURL,
		AuthType:  r.AuthType,
		Secret:    r.Secret,
		TimeoutMs: r.TimeoutMs,
	}
}

// UpdateConfigRequest is the body of PATCH /v1/providerhub/config.
type UpdateConfigRequest struct {
	Name      *string `json:"name"`
	BaseURL   *string `json:"base_url"`
	AuthType  *string `json:"auth_type"`
	Secret    *string `json:"secret"`
	Enabled   *bool   `json:"enabled"`
	TimeoutMs *int    `json:"timeout_ms"`
}

// ToCommand maps to the service command.
func (r UpdateConfigRequest) ToCommand() phcontracts.UpdateConfig {
	return phcontracts.UpdateConfig{
		Name:      r.Name,
		BaseURL:   r.BaseURL,
		AuthType:  r.AuthType,
		Secret:    r.Secret,
		Enabled:   r.Enabled,
		TimeoutMs: r.TimeoutMs,
	}
}

// ConfigResponse is the public representation (secret masked).
type ConfigResponse struct {
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

// NewConfigResponse maps a config entity.
func NewConfigResponse(c *phentity.ProviderIntegrationConfig) ConfigResponse {
	return ConfigResponse{
		ID:        c.ID,
		TenantID:  c.TenantID,
		Name:      c.Name,
		BaseURL:   c.BaseURL,
		AuthType:  c.AuthType,
		HasSecret: c.Secret != "",
		Enabled:   c.Enabled,
		TimeoutMs: c.TimeoutMs,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

// OpenTicketRequest is the body of POST /v1/conversations/{id}/external/tickets.
type OpenTicketRequest struct {
	Subject     string `json:"subject"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
}

// ToInput maps to the gateway input.
func (r OpenTicketRequest) ToInput() phcontracts.OpenTicketInput {
	return phcontracts.OpenTicketInput{Subject: r.Subject, Description: r.Description, Priority: r.Priority}
}

// Package monitoring holds the request/response DTOs for the monitoring
// endpoints. The config secret is never returned (masked); external query
// payloads pass through the normalized domain DTOs and are never persisted.
package monitoring

import (
	"time"

	mcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/monitoring/contracts"
	mentity "github.com/romerito007/chat-smsnet-omnichannel/domain/monitoring/entity"
)

// SaveConfigRequest is the body of PATCH /v1/monitoring/config (upsert).
type SaveConfigRequest struct {
	Name      *string `json:"name"`
	BaseURL   *string `json:"base_url"`
	AuthType  *string `json:"auth_type"`
	Secret    *string `json:"secret"`
	Enabled   *bool   `json:"enabled"`
	TimeoutMs *int    `json:"timeout_ms"`
}

// ToCommand maps to the service command.
func (r SaveConfigRequest) ToCommand() mcontracts.SaveConfig {
	return mcontracts.SaveConfig{
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
func NewConfigResponse(c *mentity.MonitoringIntegrationConfig) ConfigResponse {
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

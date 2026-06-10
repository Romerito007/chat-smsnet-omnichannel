// Package tenant holds the request/response DTOs for the tenant endpoints.
package tenant

import (
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/tenant/entity"
)

// TenantResponse is the public representation of a tenant.
type TenantResponse struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Status    string         `json:"status"`
	Settings  map[string]any `json:"settings,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// NewTenantResponse maps a tenant entity to its DTO.
func NewTenantResponse(t *entity.Tenant) TenantResponse {
	return TenantResponse{
		ID:        t.ID,
		Name:      t.Name,
		Status:    string(t.Status),
		Settings:  t.Settings,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
}

// UpdateTenantRequest is the body of PATCH /v1/tenants/current.
type UpdateTenantRequest struct {
	Name     string         `json:"name"`
	Settings map[string]any `json:"settings"`
}

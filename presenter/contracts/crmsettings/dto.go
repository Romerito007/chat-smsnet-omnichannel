// Package crmsettings holds the request/response DTOs for the CRM-settings endpoints.
package crmsettings

import (
	"time"

	ccontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/crmsettings/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/crmsettings/entity"
)

// CRMSettingsResponse is the public representation of a tenant's CRM module config.
type CRMSettingsResponse struct {
	TenantID        string    `json:"tenant_id"`
	TasksEnabled    bool      `json:"tasks_enabled"`
	ProductsEnabled bool      `json:"products_enabled"`
	TimelineEnabled bool      `json:"timeline_enabled"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// NewCRMSettingsResponse maps the entity to the DTO.
func NewCRMSettingsResponse(s *entity.CRMSettings) CRMSettingsResponse {
	return CRMSettingsResponse{
		TenantID:        s.TenantID,
		TasksEnabled:    s.TasksEnabled,
		ProductsEnabled: s.ProductsEnabled,
		TimelineEnabled: s.TimelineEnabled,
		UpdatedAt:       s.UpdatedAt,
	}
}

// UpdateCRMSettingsRequest is the body of PATCH /v1/crm/settings. Nil = unchanged.
type UpdateCRMSettingsRequest struct {
	TasksEnabled    *bool `json:"tasks_enabled"`
	ProductsEnabled *bool `json:"products_enabled"`
	TimelineEnabled *bool `json:"timeline_enabled"`
}

// ToCommand maps the request to the service command.
func (r UpdateCRMSettingsRequest) ToCommand() ccontracts.UpdateCRMSettings {
	return ccontracts.UpdateCRMSettings{
		TasksEnabled:    r.TasksEnabled,
		ProductsEnabled: r.ProductsEnabled,
		TimelineEnabled: r.TimelineEnabled,
	}
}

// Package sectors holds the request/response DTOs for the sector endpoints.
package sectors

import (
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/sectors/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/sectors/entity"
)

// CreateSectorRequest is the body of POST /v1/sectors.
type CreateSectorRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     *bool  `json:"enabled"`
}

// ToCommand maps the request to the service command.
func (r CreateSectorRequest) ToCommand() contracts.CreateSector {
	return contracts.CreateSector{
		Name:        r.Name,
		Description: r.Description,
		Enabled:     r.Enabled,
	}
}

// UpdateSectorRequest is the body of PATCH /v1/sectors/{id}.
type UpdateSectorRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Enabled     *bool   `json:"enabled"`
}

// ToCommand maps the request to the service command.
func (r UpdateSectorRequest) ToCommand() contracts.UpdateSector {
	return contracts.UpdateSector{
		Name:        r.Name,
		Description: r.Description,
		Enabled:     r.Enabled,
	}
}

// SectorResponse is the public representation of a sector.
type SectorResponse struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// NewSectorResponse maps a sector entity to its DTO.
func NewSectorResponse(s *entity.Sector) SectorResponse {
	return SectorResponse{
		ID:          s.ID,
		TenantID:    s.TenantID,
		Name:        s.Name,
		Description: s.Description,
		Enabled:     s.Enabled,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}

// NewSectorResponses maps a slice of sectors.
func NewSectorResponses(sectors []*entity.Sector) []SectorResponse {
	out := make([]SectorResponse, len(sectors))
	for i, s := range sectors {
		out[i] = NewSectorResponse(s)
	}
	return out
}

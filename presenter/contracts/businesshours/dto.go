// Package businesshours holds the request/response DTOs for the holidays and
// business-status endpoints.
package businesshours

import (
	"time"

	bhcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/businesshours/contracts"
	bhentity "github.com/romerito007/chat-smsnet-omnichannel/domain/businesshours/entity"
)

// CreateHolidayRequest is the body of POST /v1/holidays. An empty channel_ids
// makes the holiday apply to all channels.
type CreateHolidayRequest struct {
	Date       string   `json:"date"`
	Name       string   `json:"name"`
	ChannelIDs []string `json:"channel_ids"`
	Recurring  *bool    `json:"recurring"`
}

func (r CreateHolidayRequest) ToCommand() bhcontracts.CreateHoliday {
	return bhcontracts.CreateHoliday{Date: r.Date, Name: r.Name, ChannelIDs: r.ChannelIDs, Recurring: r.Recurring}
}

// UpdateHolidayRequest is the body of PATCH /v1/holidays/{id}.
type UpdateHolidayRequest struct {
	Date       *string   `json:"date"`
	Name       *string   `json:"name"`
	ChannelIDs *[]string `json:"channel_ids"`
	Recurring  *bool     `json:"recurring"`
}

func (r UpdateHolidayRequest) ToCommand() bhcontracts.UpdateHoliday {
	return bhcontracts.UpdateHoliday{Date: r.Date, Name: r.Name, ChannelIDs: r.ChannelIDs, Recurring: r.Recurring}
}

// HolidayResponse is the public representation of a holiday.
type HolidayResponse struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	Date       string    `json:"date"`
	Name       string    `json:"name"`
	Scope      string    `json:"scope"`
	ChannelIDs []string  `json:"channel_ids,omitempty"`
	Recurring  bool      `json:"recurring"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// NewHolidayResponse maps a holiday entity.
func NewHolidayResponse(h *bhentity.Holiday) HolidayResponse {
	return HolidayResponse{
		ID: h.ID, TenantID: h.TenantID, Date: h.Date, Name: h.Name,
		Scope: string(h.Scope), ChannelIDs: h.ChannelIDs, Recurring: h.Recurring,
		CreatedAt: h.CreatedAt, UpdatedAt: h.UpdatedAt,
	}
}

// NewHolidayResponses maps a slice.
func NewHolidayResponses(items []*bhentity.Holiday) []HolidayResponse {
	out := make([]HolidayResponse, 0, len(items))
	for _, h := range items {
		out = append(out, NewHolidayResponse(h))
	}
	return out
}

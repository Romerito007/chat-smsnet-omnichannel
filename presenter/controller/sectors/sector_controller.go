// Package sectors holds the HTTP controller for the sector endpoints.
package sectors

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	sectorservice "github.com/romerito007/chat-smsnet-omnichannel/domain/sectors/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/sectors"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves CRUD for sectors.
type Controller struct {
	sectors *sectorservice.Service
}

// NewController builds the controller.
func NewController(sectors *sectorservice.Service) *Controller {
	return &Controller{sectors: sectors}
}

// Create handles POST /v1/sectors.
func (c *Controller) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateSectorRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	s, err := c.sectors.Create(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, dto.NewSectorResponse(s))
}

// List handles GET /v1/sectors.
func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	page := middleware.PageFromRequest(r)
	items, err := c.sectors.List(r.Context(), page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	resp := shared.NewPage(dto.NewSectorResponses(items), page.Limit, func(s dto.SectorResponse) shared.Cursor {
		return shared.Cursor{CreatedAt: s.CreatedAt.UnixMilli(), ID: s.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}

// Get handles GET /v1/sectors/{id}.
func (c *Controller) Get(w http.ResponseWriter, r *http.Request) {
	s, err := c.sectors.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewSectorResponse(s))
}

// Update handles PATCH /v1/sectors/{id}.
func (c *Controller) Update(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateSectorRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	s, err := c.sectors.Update(r.Context(), chi.URLParam(r, "id"), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewSectorResponse(s))
}

// Delete handles DELETE /v1/sectors/{id}.
func (c *Controller) Delete(w http.ResponseWriter, r *http.Request) {
	if err := c.sectors.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

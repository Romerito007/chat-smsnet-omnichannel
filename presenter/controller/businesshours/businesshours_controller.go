// Package businesshours holds the HTTP controllers for holidays and the sector
// business-status endpoint.
package businesshours

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	bhservice "github.com/romerito007/chat-smsnet-omnichannel/domain/businesshours/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/businesshours"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves holiday CRUD and the sector business-status query.
type Controller struct {
	holidays *bhservice.HolidayService
	hours    *bhservice.BusinessHoursService
}

// NewController builds the controller.
func NewController(holidays *bhservice.HolidayService, hours *bhservice.BusinessHoursService) *Controller {
	return &Controller{holidays: holidays, hours: hours}
}

// ── holidays ─────────────────────────────────────────────────────────────────

func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	page := middleware.PageFromRequest(r)
	items, err := c.holidays.List(r.Context(), page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	resp := shared.NewPage(dto.NewHolidayResponses(items), page.Limit, func(h dto.HolidayResponse) shared.Cursor {
		return shared.Cursor{CreatedAt: h.CreatedAt.UnixMilli(), ID: h.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}

func (c *Controller) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateHolidayRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	h, err := c.holidays.Create(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, dto.NewHolidayResponse(h))
}

func (c *Controller) Get(w http.ResponseWriter, r *http.Request) {
	h, err := c.holidays.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewHolidayResponse(h))
}

func (c *Controller) Update(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateHolidayRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	h, err := c.holidays.Update(r.Context(), chi.URLParam(r, "id"), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewHolidayResponse(h))
}

func (c *Controller) Delete(w http.ResponseWriter, r *http.Request) {
	if err := c.holidays.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── business status ──────────────────────────────────────────────────────────

// BusinessStatus handles GET /v1/sectors/{id}/business-status. An optional ?at=
// RFC3339 instant overrides "now" (useful for previews/testing).
func (c *Controller) BusinessStatus(w http.ResponseWriter, r *http.Request) {
	at := time.Now()
	if raw := r.URL.Query().Get("at"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			middleware.WriteError(w, r, apperror.Validation("at must be an RFC3339 timestamp"))
			return
		}
		at = parsed
	}
	status, err := c.hours.Status(r.Context(), chi.URLParam(r, "id"), at)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, status)
}

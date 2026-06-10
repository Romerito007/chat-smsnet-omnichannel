// Package webhooks holds the HTTP controllers for webhook subscription
// management and delivery history.
package webhooks

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	wservice "github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/service"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/webhooks"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves webhook CRUD, test deliveries and delivery history.
type Controller struct {
	svc *wservice.SubscriptionService
}

// NewController builds the controller.
func NewController(svc *wservice.SubscriptionService) *Controller {
	return &Controller{svc: svc}
}

// List handles GET /v1/webhooks with keyset pagination.
func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	page := middleware.PageFromRequest(r)
	items, err := c.svc.List(r.Context(), page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	resp := shared.NewPage(dto.NewSubscriptionResponses(items), page.Limit, func(s dto.SubscriptionResponse) shared.Cursor {
		return shared.Cursor{CreatedAt: s.CreatedAt.UnixMilli(), ID: s.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}

// Create handles POST /v1/webhooks. The response includes the secret once.
func (c *Controller) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	sub, err := c.svc.Create(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, dto.NewCreatedResponse(sub))
}

// Get handles GET /v1/webhooks/{id}.
func (c *Controller) Get(w http.ResponseWriter, r *http.Request) {
	sub, err := c.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewSubscriptionResponse(sub))
}

// Update handles PATCH /v1/webhooks/{id}.
func (c *Controller) Update(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	sub, err := c.svc.Update(r.Context(), chi.URLParam(r, "id"), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewSubscriptionResponse(sub))
}

// Delete handles DELETE /v1/webhooks/{id}.
func (c *Controller) Delete(w http.ResponseWriter, r *http.Request) {
	if err := c.svc.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Test handles POST /v1/webhooks/{id}/test.
func (c *Controller) Test(w http.ResponseWriter, r *http.Request) {
	res, err := c.svc.Test(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, res)
}

// Deliveries handles GET /v1/webhooks/{id}/deliveries with keyset pagination.
func (c *Controller) Deliveries(w http.ResponseWriter, r *http.Request) {
	page := middleware.PageFromRequest(r)
	items, err := c.svc.ListDeliveries(r.Context(), chi.URLParam(r, "id"), page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	resp := shared.NewPage(dto.NewDeliveryResponses(items), page.Limit, func(d dto.DeliveryResponse) shared.Cursor {
		return shared.Cursor{CreatedAt: d.CreatedAt.UnixMilli(), ID: d.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}

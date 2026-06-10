// Package tenant holds the HTTP controller for the tenant endpoints.
package tenant

import (
	"net/http"

	tenantservice "github.com/romerito007/chat-smsnet-omnichannel/domain/tenant/service"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/tenant"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves the current-tenant endpoints.
type Controller struct {
	tenants *tenantservice.Service
}

// NewController builds the controller.
func NewController(tenants *tenantservice.Service) *Controller {
	return &Controller{tenants: tenants}
}

// Current handles GET /v1/tenants/current.
func (c *Controller) Current(w http.ResponseWriter, r *http.Request) {
	t, err := c.tenants.Current(r.Context())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewTenantResponse(t))
}

// Update handles PATCH /v1/tenants/current.
func (c *Controller) Update(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateTenantRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	t, err := c.tenants.UpdateSettings(r.Context(), req.Name, req.Settings)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewTenantResponse(t))
}

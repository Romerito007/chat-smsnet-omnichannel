// Package crmsettings holds the HTTP controller for the per-tenant CRM settings
// (optional module toggles) endpoints.
package crmsettings

import (
	"net/http"

	crmservice "github.com/romerito007/chat-smsnet-omnichannel/domain/crmsettings/service"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/crmsettings"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves the CRM-settings read and write. Tenant-scoped via the token.
type Controller struct {
	settings *crmservice.Service
}

// NewController builds the controller.
func NewController(settings *crmservice.Service) *Controller {
	return &Controller{settings: settings}
}

// Get handles GET /v1/crm/settings (deal.view): the tenant's module toggles, or the
// conservative defaults when never configured.
func (c *Controller) Get(w http.ResponseWriter, r *http.Request) {
	s, err := c.settings.Get(r.Context())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewCRMSettingsResponse(s))
}

// Update handles PATCH /v1/crm/settings (crm.manage): toggle the optional modules.
func (c *Controller) Update(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateCRMSettingsRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	s, err := c.settings.Update(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewCRMSettingsResponse(s))
}

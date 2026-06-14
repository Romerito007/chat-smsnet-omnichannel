// Package platform holds the HTTP controller for the platform-plane provisioning
// endpoint. It is mounted behind PlatformAuth (X-Platform-Key), above tenant
// isolation, and creates a tenant + owner — nothing else.
package platform

import (
	"net/http"

	platformservice "github.com/romerito007/chat-smsnet-omnichannel/domain/platform/service"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/platform"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves tenant provisioning.
type Controller struct {
	svc *platformservice.Service
}

// NewController builds the controller.
func NewController(svc *platformservice.Service) *Controller {
	return &Controller{svc: svc}
}

// ProvisionTenant handles POST /v1/platform/tenants. The platform key id (for
// audit) is taken from the authenticated platform context. On a repeated
// external_ref it returns 200 with the existing tenant; on a new one, 201.
func (c *Controller) ProvisionTenant(w http.ResponseWriter, r *http.Request) {
	var req dto.ProvisionTenantRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	keyID := middleware.PlatformKeyID(r.Context())
	res, err := c.svc.Provision(r.Context(), req.ToCommand(keyID))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	status := http.StatusCreated
	if !res.Created {
		status = http.StatusOK
	}
	middleware.WriteJSON(w, status, dto.NewProvisionResponse(res))
}

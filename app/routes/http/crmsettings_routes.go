package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerCRMSettingsRoutes mounts the per-tenant CRM settings (optional-module
// toggles), tenant-scoped (the tenant comes from the access token). Reading is open
// to any CRM user (deal.view); toggling modules requires crm.manage.
func registerCRMSettingsRoutes(r chi.Router, c *container.Container) {
	ctl := factories.CRMSettingsController(c)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))

		p.With(middleware.RequirePermission(authz.DealView)).Get("/crm/settings", ctl.Get)
		p.With(middleware.RequirePermission(authz.CRMManage)).Patch("/crm/settings", ctl.Update)
	})
}

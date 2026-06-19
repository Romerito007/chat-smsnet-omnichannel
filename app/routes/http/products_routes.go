package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerProductRoutes mounts the CRM product-catalog endpoints, tenant-scoped (the
// tenant comes from the token). Reading is open to any CRM user (deal.view);
// creating/editing/deactivating requires crm.manage. All respect the tenant's
// products toggle (crmsettings.products_enabled).
func registerProductRoutes(r chi.Router, c *container.Container) {
	ctl := factories.ProductController(c)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))

		p.With(middleware.RequirePermission(authz.DealView)).Get("/crm/products", ctl.List)
		p.With(middleware.RequirePermission(authz.CRMManage)).Post("/crm/products", ctl.Create)
		p.With(middleware.RequirePermission(authz.CRMManage)).Patch("/crm/products/{id}", ctl.Update)
	})
}

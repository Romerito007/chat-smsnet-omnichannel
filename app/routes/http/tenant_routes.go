package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerTenantRoutes mounts the current-tenant endpoints (authenticated).
// Reading is available to any authenticated user; updating requires user.manage
// (the admin/owner capability).
func registerTenantRoutes(r chi.Router, c *container.Container) {
	ctl := factories.TenantController(c)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))
		p.Get("/tenants/current", ctl.Current)
		p.With(middleware.RequirePermission(authz.UserManage)).Patch("/tenants/current", ctl.Update)
	})
}

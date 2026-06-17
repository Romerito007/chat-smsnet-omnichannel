package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerGroupRoutes mounts the WhatsApp groups management endpoints, tenant-scoped
// (the tenant comes from the access token). Reads require group.view; the attend
// toggle and the gateway sync require group.manage.
func registerGroupRoutes(r chi.Router, c *container.Container) {
	ctl := factories.GroupController(c)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))

		p.With(middleware.RequirePermission(authz.GroupView)).Get("/groups", ctl.List)
		p.With(middleware.RequirePermission(authz.GroupManage)).Patch("/groups/{id}", ctl.SetAttend)
		p.With(middleware.RequirePermission(authz.GroupManage)).Post("/groups/sync", ctl.Sync)
	})
}

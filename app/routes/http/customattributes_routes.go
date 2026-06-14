package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerCustomAttributeRoutes mounts custom-attribute definition CRUD. Reads are
// open to any authenticated user (the value form needs the definitions to render);
// writes require customattribute.manage.
func registerCustomAttributeRoutes(r chi.Router, c *container.Container) {
	ctl := factories.CustomAttributeController(c)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))

		p.Get("/custom-attributes", ctl.List)
		p.Get("/custom-attributes/{id}", ctl.Get)

		p.With(middleware.RequirePermission(authz.CustomAttributeManage)).Post("/custom-attributes", ctl.Create)
		p.With(middleware.RequirePermission(authz.CustomAttributeManage)).Patch("/custom-attributes/{id}", ctl.Update)
		p.With(middleware.RequirePermission(authz.CustomAttributeManage)).Delete("/custom-attributes/{id}", ctl.Delete)
	})
}

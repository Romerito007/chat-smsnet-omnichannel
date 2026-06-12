package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerContactRoutes mounts the contact CRM endpoints, tenant-scoped (the
// tenant comes from the access token). Reads require contact.read; writes
// (create/update) require contact.write.
func registerContactRoutes(r chi.Router, c *container.Container) {
	ctl := factories.ContactController(c)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))

		p.With(middleware.RequirePermission(authz.ContactRead)).Get("/contacts", ctl.List)
		p.With(middleware.RequirePermission(authz.ContactRead)).Get("/contacts/{id}", ctl.Get)
		p.With(middleware.RequirePermission(authz.ContactWrite)).Post("/contacts", ctl.Create)
		p.With(middleware.RequirePermission(authz.ContactWrite)).Patch("/contacts/{id}", ctl.Update)
	})
}

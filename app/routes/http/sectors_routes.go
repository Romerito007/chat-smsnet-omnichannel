package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerSectorRoutes mounts sector CRUD, gated on sector.manage.
func registerSectorRoutes(r chi.Router, c *container.Container) {
	ctl := factories.SectorController(c)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))
		p.Use(middleware.RequirePermission(authz.SectorManage))

		p.Route("/sectors", func(s chi.Router) {
			s.Get("/", ctl.List)
			s.Post("/", ctl.Create)
			s.Get("/{id}", ctl.Get)
			s.Patch("/{id}", ctl.Update)
			s.Delete("/{id}", ctl.Delete)
		})
	})
}

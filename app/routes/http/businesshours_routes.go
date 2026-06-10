package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerBusinessHoursRoutes mounts holiday CRUD and the sector business-status
// query, all gated on sector.manage. The business-status endpoint mounts as its
// own subrouter alongside the existing /sectors mount.
func registerBusinessHoursRoutes(r chi.Router, c *container.Container) {
	ctl := factories.BusinessHoursController(c)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))
		p.Use(middleware.RequirePermission(authz.SectorManage))

		p.Route("/holidays", func(h chi.Router) {
			h.Get("/", ctl.List)
			h.Post("/", ctl.Create)
			h.Get("/{id}", ctl.Get)
			h.Patch("/{id}", ctl.Update)
			h.Delete("/{id}", ctl.Delete)
		})

		p.Route("/sectors/{id}/business-status", func(bs chi.Router) {
			bs.Get("/", ctl.BusinessStatus)
		})
	})
}

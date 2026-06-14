package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerBusinessHoursRoutes mounts holiday CRUD (sector.manage) and the channel
// business-status query (channel.manage). Business hours live on the channel.
func registerBusinessHoursRoutes(r chi.Router, c *container.Container) {
	ctl := factories.BusinessHoursController(c)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))

		p.With(middleware.RequirePermission(authz.SectorManage)).Route("/holidays", func(h chi.Router) {
			h.Get("/", ctl.List)
			h.Post("/", ctl.Create)
			h.Get("/{id}", ctl.Get)
			h.Patch("/{id}", ctl.Update)
			h.Delete("/{id}", ctl.Delete)
		})

		p.With(middleware.RequirePermission(authz.ChannelManage)).
			Get("/channels/{id}/business-status", ctl.BusinessStatus)
	})
}

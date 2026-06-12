package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerSectorRoutes mounts sector endpoints: reads (list/get) are available to
// any authenticated tenant user that can handle conversations (conversation.read),
// since agents need the sector list for filters and assignment; writes require
// sector.manage.
func registerSectorRoutes(r chi.Router, c *container.Container) {
	ctl := factories.SectorController(c)

	read := middleware.RequirePermission(authz.ConversationRead)
	manage := middleware.RequirePermission(authz.SectorManage)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))

		p.Route("/sectors", func(s chi.Router) {
			s.With(read).Get("/", ctl.List)
			s.With(read).Get("/{id}", ctl.Get)
			s.With(manage).Post("/", ctl.Create)
			s.With(manage).Patch("/{id}", ctl.Update)
			s.With(manage).Delete("/{id}", ctl.Delete)
		})
	})
}

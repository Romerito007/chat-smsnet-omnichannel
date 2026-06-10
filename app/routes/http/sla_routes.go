package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerSLARoutes mounts SLA policy CRUD (sector.manage), the at-risk listing
// and the per-conversation SLA status (conversation.read). The conversation SLA
// status mounts as its own subrouter alongside the existing /conversations
// mounts.
func registerSLARoutes(r chi.Router, c *container.Container) {
	ctl := factories.SLAController(c)

	manage := middleware.RequirePermission(authz.SectorManage)
	read := middleware.RequirePermission(authz.ConversationRead)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))

		p.Route("/sla/policies", func(sp chi.Router) {
			sp.With(manage).Get("/", ctl.ListPolicies)
			sp.With(manage).Post("/", ctl.CreatePolicy)
			sp.With(manage).Get("/{id}", ctl.GetPolicy)
			sp.With(manage).Patch("/{id}", ctl.UpdatePolicy)
			sp.With(manage).Delete("/{id}", ctl.DeletePolicy)
		})

		p.With(read).Get("/sla/at-risk", ctl.AtRisk)

		p.Route("/conversations/{id}/sla", func(cs chi.Router) {
			cs.With(read).Get("/", ctl.ConversationSLA)
		})
	})
}

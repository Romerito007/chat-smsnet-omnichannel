package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerRoutingRoutes mounts the routing endpoints. Assignment/enqueue require
// conversation.assign; transfer requires conversation.transfer. The service
// additionally enforces per-agent visibility and agent eligibility.
func registerRoutingRoutes(r chi.Router, c *container.Container) {
	ctl := factories.RoutingController(c)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))

		p.With(middleware.RequirePermission(authz.ConversationAssign)).
			Post("/conversations/{id}/assign", ctl.Assign)
		p.With(middleware.RequirePermission(authz.ConversationTransfer)).
			Post("/conversations/{id}/transfer", ctl.Transfer)
		p.With(middleware.RequirePermission(authz.ConversationAssign)).
			Post("/conversations/{id}/enqueue", ctl.Enqueue)
		p.With(middleware.RequirePermission(authz.ConversationAssign)).
			Post("/routing/run", ctl.Run)
	})
}

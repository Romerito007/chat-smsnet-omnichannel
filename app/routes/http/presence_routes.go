package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerPresenceRoutes mounts the agent presence endpoints. Both require
// authentication; the service enforces that an agent may only change its own
// status unless it holds user.manage (supervisor/admin). This own-resource rule
// is covered by TestSetStatus_CannotChangeOthersWithoutPermission
// (domain/presence/service).
func registerPresenceRoutes(r chi.Router, c *container.Container) {
	ctl := factories.PresenceController(c)
	agents := factories.AgentsController(c)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))
		p.Get("/agents/presence", ctl.List)
		p.Post("/agents/presence/status", ctl.SetStatus)
		// Assignable-agents directory for the assignment selector (id, name,
		// presence). Lighter than /v1/users and readable by anyone who can assign.
		p.With(middleware.RequirePermission(authz.ConversationAssign)).Get("/agents", agents.List)
	})
}

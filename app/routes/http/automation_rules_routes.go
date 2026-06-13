package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerAutomationRulesRoutes mounts the automation-rules CRUD. Every route
// requires automation.manage; the tenant is derived from the access token. This
// is the Chatwoot-style rules engine, distinct from the /automation external-flow
// orchestration.
func registerAutomationRulesRoutes(r chi.Router, c *container.Container) {
	ctl := factories.AutomationRuleController(c)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))
		p.Use(middleware.RequirePermission(authz.AutomationManage))

		p.Route("/automation-rules", func(ar chi.Router) {
			ar.Get("/", ctl.List)
			ar.Post("/", ctl.Create)
			ar.Get("/{id}", ctl.Get)
			ar.Patch("/{id}", ctl.Update)
			ar.Delete("/{id}", ctl.Delete)
		})
	})
}

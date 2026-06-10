package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerAutomationRoutes mounts automation integration CRUD + run reads
// (automation.manage) and the public, signature-verified flow callback.
func registerAutomationRoutes(r chi.Router, c *container.Container) {
	ctl := factories.AutomationController(c)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))
		p.Use(middleware.RequirePermission(authz.AutomationManage))

		p.Route("/automation/integrations", func(in chi.Router) {
			in.Get("/", ctl.ListIntegrations)
			in.Post("/", ctl.CreateIntegration)
			in.Get("/{id}", ctl.GetIntegration)
			in.Patch("/{id}", ctl.UpdateIntegration)
			in.Delete("/{id}", ctl.DeleteIntegration)
		})

		p.Get("/automation/runs", ctl.ListRuns)
		p.Get("/automation/runs/{id}", ctl.GetRun)
	})

	// Public callback from the external flow (signature-verified in the service).
	r.Post("/automation/callbacks/{tenant_id}", ctl.Callback)
}

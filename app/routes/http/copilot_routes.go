package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerCopilotRoutes mounts the copilot config endpoints (copilot.configure)
// and the inference endpoints (copilot.use). The tenant is derived from the
// access token; the service enforces conversation visibility and the
// allow_*_data privacy policies.
func registerCopilotRoutes(r chi.Router, c *container.Container) {
	ctl := factories.CopilotController(c)
	asst := factories.CopilotAssistantController(c)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))

		// Configuration.
		p.With(middleware.RequirePermission(authz.CopilotConfigure)).Get("/copilot/config", ctl.GetConfig)
		p.With(middleware.RequirePermission(authz.CopilotConfigure)).Patch("/copilot/config", ctl.SaveConfig)

		// Assistants (many per tenant): manage with copilot.configure.
		p.With(middleware.RequirePermission(authz.CopilotConfigure)).Get("/copilot/assistants", asst.List)
		p.With(middleware.RequirePermission(authz.CopilotConfigure)).Post("/copilot/assistants", asst.Create)
		p.With(middleware.RequirePermission(authz.CopilotConfigure)).Get("/copilot/assistants/{id}", asst.Get)
		p.With(middleware.RequirePermission(authz.CopilotConfigure)).Patch("/copilot/assistants/{id}", asst.Update)
		p.With(middleware.RequirePermission(authz.CopilotConfigure)).Delete("/copilot/assistants/{id}", asst.Delete)

		// Inference.
		p.With(middleware.RequirePermission(authz.CopilotUse)).Post("/copilot/suggest-reply", ctl.SuggestReply)
		p.With(middleware.RequirePermission(authz.CopilotUse)).Post("/copilot/ask", ctl.Ask)
		p.With(middleware.RequirePermission(authz.CopilotUse)).Post("/copilot/summarize", ctl.Summarize)
		p.With(middleware.RequirePermission(authz.CopilotUse)).Post("/copilot/classify", ctl.Classify)
		p.With(middleware.RequirePermission(authz.CopilotUse)).Post("/copilot/next-action", ctl.NextAction)
	})
}

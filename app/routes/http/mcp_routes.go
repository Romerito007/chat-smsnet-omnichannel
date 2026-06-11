package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerMCPRoutes mounts the MCP server configuration (integration.configure)
// and the conversation-scoped tool/approval endpoints. Read runs require
// integration.read; any write (manual or AI-proposed) requires
// integration.execute_action and explicit confirmation.
func registerMCPRoutes(r chi.Router, c *container.Container) {
	servers := factories.MCPServerController(c)
	tools := factories.MCPToolController(c)

	// Server registry CRUD + test (admin configuration).
	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))
		p.Use(middleware.RequirePermission(authz.IntegrationConfigure))

		p.Route("/mcp/servers", func(s chi.Router) {
			s.Get("/", servers.List)
			s.Post("/", servers.Create)
			s.Get("/{id}", servers.Get)
			s.Patch("/{id}", servers.Update)
			s.Delete("/{id}", servers.Delete)
			s.Post("/{id}/test", servers.Test)
		})
	})

	// Conversation-scoped tool usage by the agent (subrouters mirror the external
	// routes pattern so they nest cleanly under /conversations/{id}).
	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))

		// Discovery + read execution require integration.read; the service also
		// enforces integration.execute_action for write tools.
		p.Route("/conversations/{id}/mcp", func(m chi.Router) {
			m.Use(middleware.RequirePermission(authz.IntegrationRead))
			m.Get("/tools", tools.List)
			m.Post("/run", tools.Run)
		})

		// Approving/rejecting a proposed write action requires execute_action.
		p.Route("/conversations/{id}/copilot/approvals", func(a chi.Router) {
			a.Use(middleware.RequirePermission(authz.IntegrationExecuteAction))
			a.Post("/{approvalID}", tools.Decide)
		})
	})
}

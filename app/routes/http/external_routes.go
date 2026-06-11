package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerExternalRoutes mounts the on-demand, by-conversation queries to the
// smsnet-integrations API under a single /conversations/{id}/external subrouter.
// The base requires conversation.read (visibility); reads add integration.read,
// and the side-effect actions (liberacao/chamado) require
// integration.execute_action and are audited. The service additionally enforces
// conversation visibility + per-tenant rate limiting and persists no payload.
func registerExternalRoutes(r chi.Router, c *container.Container) {
	provider := factories.ProviderHubController(c)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))

		p.Route("/conversations/{id}/external", func(ex chi.Router) {
			ex.Use(middleware.RequirePermission(authz.ConversationRead))

			read := middleware.RequirePermission(authz.IntegrationRead)
			ex.With(read).Get("/cliente", provider.Cliente)
			ex.With(read).Get("/planos", provider.Planos)
			ex.With(read).Get("/empresa", provider.Empresa)

			act := middleware.RequirePermission(authz.IntegrationExecuteAction)
			ex.With(act).Post("/liberacao", provider.Liberacao)
			ex.With(act).Post("/chamado", provider.Chamado)
		})
	})
}

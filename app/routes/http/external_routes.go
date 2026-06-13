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

			// Reads are POST so the body can carry isp_config_id (the ISP profile to
			// use) alongside the lookup fields.
			read := middleware.RequirePermission(authz.IntegrationRead)
			ex.With(read).Post("/cliente", provider.Cliente)
			ex.With(read).Post("/planos", provider.Planos)
			ex.With(read).Post("/empresa", provider.Empresa)

			// Side-effect actions: idempotency-keyed (replayed + forwarded to the
			// gateway for upstream dedup) and audited.
			act := middleware.RequirePermission(authz.IntegrationExecuteAction)
			idem := middleware.Idempotency(c.Redis)
			ex.With(act, idem).Post("/liberacao", provider.Liberacao)
			ex.With(act, idem).Post("/chamado", provider.Chamado)
		})
	})
}

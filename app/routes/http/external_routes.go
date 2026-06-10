package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerExternalRoutes mounts the on-demand, by-conversation queries to
// external systems (providerhub + monitoring) under a single
// /conversations/{id}/external subrouter. chi mounts a subrouter per path, so
// every external integration must register here rather than mounting the same
// path twice. Each handler carries its own permission requirements; the
// services also enforce conversation visibility and per-tenant rate limiting,
// and never persist the external payloads they return.
func registerExternalRoutes(r chi.Router, c *container.Container) {
	provider := factories.ProviderHubController(c)
	monitoring := factories.MonitoringController(c)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))

		p.Route("/conversations/{id}/external", func(ex chi.Router) {
			// Every external query requires reading the conversation.
			ex.Use(middleware.RequirePermission(authz.ConversationRead))

			// providerhub: standardized provider data.
			ex.Get("/customer-profile", provider.CustomerProfile)
			ex.Get("/contracts", provider.Contracts)
			ex.With(middleware.RequirePermission(authz.ContactViewFinancial)).
				Get("/financial-status", provider.FinancialStatus)
			ex.With(middleware.RequirePermission(authz.ContactViewConnectionStatus)).
				Get("/connection-status", provider.ConnectionStatus)
			ex.Get("/tickets", provider.Tickets)
			ex.With(middleware.RequirePermission(authz.IntegrationExecuteAction)).
				Post("/tickets", provider.OpenTicket)

			// monitoring: technical status, requires connection-status permission.
			ex.With(middleware.RequirePermission(authz.ContactViewConnectionStatus)).
				Get("/monitoring-summary", monitoring.Summary)
			ex.With(middleware.RequirePermission(authz.ContactViewConnectionStatus)).
				Get("/monitoring-incidents", monitoring.Incidents)
		})
	})
}

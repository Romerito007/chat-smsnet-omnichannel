package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerProviderHubRoutes mounts the providerhub config endpoints and the
// on-demand conversation query endpoints. Financial/connection/open-ticket carry
// extra permission requirements; the service also enforces conversation
// visibility and per-tenant rate limiting.
func registerProviderHubRoutes(r chi.Router, c *container.Container) {
	ctl := factories.ProviderHubController(c)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))

		// Config management.
		p.With(middleware.RequirePermission(authz.IntegrationRead)).Get("/providerhub/config", ctl.GetConfig)
		p.With(middleware.RequirePermission(authz.IntegrationConfigure)).Post("/providerhub/config", ctl.CreateConfig)
		p.With(middleware.RequirePermission(authz.IntegrationConfigure)).Patch("/providerhub/config", ctl.UpdateConfig)
		p.With(middleware.RequirePermission(authz.IntegrationConfigure)).Post("/providerhub/config/test", ctl.TestConfig)

		// On-demand queries by conversation.
		p.Route("/conversations/{id}/external", func(ex chi.Router) {
			ex.With(middleware.RequirePermission(authz.ConversationRead)).
				Get("/customer-profile", ctl.CustomerProfile)
			ex.With(middleware.RequirePermission(authz.ConversationRead)).
				Get("/contracts", ctl.Contracts)
			ex.With(middleware.RequirePermission(authz.ConversationRead)).
				With(middleware.RequirePermission(authz.ContactViewFinancial)).
				Get("/financial-status", ctl.FinancialStatus)
			ex.With(middleware.RequirePermission(authz.ConversationRead)).
				With(middleware.RequirePermission(authz.ContactViewConnectionStatus)).
				Get("/connection-status", ctl.ConnectionStatus)
			ex.With(middleware.RequirePermission(authz.ConversationRead)).
				Get("/tickets", ctl.Tickets)
			ex.With(middleware.RequirePermission(authz.ConversationRead)).
				With(middleware.RequirePermission(authz.IntegrationExecuteAction)).
				Post("/tickets", ctl.OpenTicket)
		})
	})
}

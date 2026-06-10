// Package http builds the API router: global middleware chain plus per-domain
// route groups mounted under /v1. Each domain has its own <domain>_routes.go.
package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/policy"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// NewRouter assembles the full HTTP router with the global middleware chain and
// all domain routes mounted under /v1.
func NewRouter(c *container.Container) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware chain (outermost first).
	r.Use(middleware.Recover(c.Logger))
	r.Use(middleware.Telemetry("http.server"))
	r.Use(middleware.RequestID(c.Logger))
	r.Use(middleware.CORS(c.Config.HTTP.AllowedOrigins))

	// Health endpoints live outside /v1 and outside rate-limit scope.
	health := factories.HealthHandler(c)
	r.Get("/healthz", health.Live)
	r.Get("/readyz", health.Ready)

	// Versioned API surface. Rate limit + idempotency apply to everything; the
	// tenant is derived per-request from the access token (never a header), so
	// there is no global tenant middleware — protected groups install AuthContext.
	r.Route("/v1", func(v1 chi.Router) {
		v1.Use(middleware.RateLimit(c.Redis, policy.DefaultAPIRateLimit))
		v1.Use(middleware.Idempotency(c.Redis))

		registerAuthRoutes(v1, c)
		registerTenantRoutes(v1, c)
		registerIAMRoutes(v1, c)
		registerSectorRoutes(v1, c)
		registerQueueRoutes(v1, c)
		registerPresenceRoutes(v1, c)
		registerConversationRoutes(v1, c)
		registerRoutingRoutes(v1, c)
		registerChannelRoutes(v1, c)
		registerAutomationRoutes(v1, c)
		registerProviderHubRoutes(v1, c)
		registerMonitoringRoutes(v1, c)
		registerWebhookRoutes(v1, c)
		registerCopilotRoutes(v1, c)
		registerConversationToolsRoutes(v1, c)
		registerBusinessHoursRoutes(v1, c)
		registerSLARoutes(v1, c)
		registerNotificationRoutes(v1, c)

		// Shared on-demand external queries (providerhub + monitoring) under a
		// single /conversations/{id}/external subrouter.
		registerExternalRoutes(v1, c)
	})

	return r
}

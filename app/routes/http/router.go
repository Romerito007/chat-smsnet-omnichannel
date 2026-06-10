// Package http builds the API router: global middleware chain plus per-domain
// route groups. Each domain adds its own <domain>_routes.go registering routes
// on the shared chi.Router.
package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/policy"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// NewRouter assembles the full HTTP router with the global middleware chain and
// all domain routes mounted under /api.
func NewRouter(c *container.Container) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware chain (outermost first). Recover is outermost so it
	// catches panics anywhere; Telemetry wraps the rest so spans/metrics cover
	// the full request; RequestID provides correlation for logs and the error
	// envelope.
	r.Use(middleware.Recover(c.Logger))
	r.Use(middleware.Telemetry("http.server"))
	r.Use(middleware.RequestID(c.Logger))
	r.Use(middleware.CORS(c.Config.HTTP.AllowedOrigins))

	// Health endpoints live outside /api and outside tenant/rate-limit scope.
	health := factories.HealthHandler(c)
	r.Get("/healthz", health.Live)
	r.Get("/readyz", health.Ready)

	// Tenant-scoped, rate-limited API surface.
	r.Route("/api", func(api chi.Router) {
		api.Use(middleware.TenantContext)
		api.Use(middleware.AuthStub)
		api.Use(middleware.RateLimit(c.Redis, policy.DefaultAPIRateLimit))
		api.Use(middleware.Idempotency(c.Redis))

		// Domain route groups are registered here as they are implemented, e.g.:
		//   conversationroutes.Register(api, c)
		registerDomainRoutes(api, c)
	})

	return r
}

// registerDomainRoutes is the single extension point where each domain's
// <domain>_routes.go hooks its routes into the /api group. It is intentionally
// empty in the foundation.
func registerDomainRoutes(_ chi.Router, _ *container.Container) {}

// Package http builds the API router: global middleware chain plus per-domain
// route groups mounted under /v1. Each domain has its own <domain>_routes.go.
package http

import (
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/policy"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/openapi"
)

// catalogCache adds ETag + a short private Cache-Control to quasi-static catalog
// reads (tags/sectors/queues/close-reasons/canned-responses, /me), so a client
// re-navigating gets a 304 instead of refetching. Applied per-handler (never to
// volatile lists like /conversations).
var catalogCache = middleware.ConditionalCache(45 * time.Second)

// NewRouter assembles the full HTTP router with the global middleware chain and
// all domain routes mounted under /v1.
func NewRouter(c *container.Container) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware chain (outermost first).
	r.Use(middleware.Recover(c.Logger))
	r.Use(middleware.Telemetry("http.server"))
	r.Use(middleware.RequestID(c.Logger, c.Config.LogRequestBody))
	r.Use(middleware.CORS(c.Config.HTTP.AllowedOrigins))

	// Health endpoints live outside /v1 and outside rate-limit scope.
	health := factories.HealthHandler(c)
	r.Get("/healthz", health.Live)
	r.Get("/readyz", health.Ready)

	// Machine-readable API contract. Public in development; behind HTTP basic auth
	// in production (the frontend generates a typed client from this).
	r.Get("/openapi.json", openapi.Handler(openapi.Config{
		Public:    c.Config.AppEnv != "production",
		BasicUser: c.Config.HTTP.OpenAPIBasicUser,
		BasicPass: c.Config.HTTP.OpenAPIBasicPass,
	}))

	// Versioned API surface. Rate limit + idempotency apply to everything; the
	// tenant is derived per-request from the access token (never a header), so
	// there is no global tenant middleware — protected groups install AuthContext.
	r.Route("/v1", func(v1 chi.Router) {
		v1.Use(middleware.RateLimit(c.Redis, policy.DefaultAPIRateLimit))
		v1.Use(middleware.Idempotency(c.Redis))

		registerAuthRoutes(v1, c)
		registerPlatformRoutes(v1, c)
		registerTenantRoutes(v1, c)
		registerIAMRoutes(v1, c)
		registerSectorRoutes(v1, c)
		registerQueueRoutes(v1, c)
		registerPresenceRoutes(v1, c)
		registerConversationRoutes(v1, c)
		registerContactRoutes(v1, c)
		registerRoutingRoutes(v1, c)
		registerChannelRoutes(v1, c)
		registerGroupRoutes(v1, c)
		registerPipelineRoutes(v1, c)
		registerDealRoutes(v1, c)
		registerAutomationRulesRoutes(v1, c)
		registerProviderHubRoutes(v1, c)
		registerWebhookRoutes(v1, c)
		registerCopilotRoutes(v1, c)
		registerMCPRoutes(v1, c)
		registerConversationToolsRoutes(v1, c)
		registerBusinessHoursRoutes(v1, c)
		registerCustomAttributeRoutes(v1, c)
		registerSLARoutes(v1, c)
		registerNotificationRoutes(v1, c)
		registerCSATRoutes(v1, c)
		registerSearchRoutes(v1, c)
		registerReportRoutes(v1, c)
		registerPrivacyRoutes(v1, c)
		registerAttachmentRoutes(v1, c)

		// On-demand external queries (providerhub) under a single
		// /conversations/{id}/external subrouter.
		registerExternalRoutes(v1, c)
	})

	return r
}

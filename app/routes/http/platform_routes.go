package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/policy"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerPlatformRoutes mounts the platform-plane provisioning endpoint. It sits
// ABOVE tenant isolation: authenticated by X-Platform-Key (PlatformAuth), NOT the
// tenant Bearer token, and it creates a tenant + owner — nothing else. A strict
// per-IP rate limit guards against a leaked-key flood. With no platform keys
// configured, PlatformAuth rejects every request (the endpoint is inert).
func registerPlatformRoutes(r chi.Router, c *container.Container) {
	ctl := factories.PlatformController(c)

	r.Group(func(p chi.Router) {
		p.Use(middleware.RateLimitScoped(c.Redis, policy.PlatformProvisionRateLimit, "platform_provision"))
		p.Use(middleware.PlatformAuth(c.Config.Platform.APIKeys))
		p.Post("/platform/tenants", ctl.ProvisionTenant)
	})
}

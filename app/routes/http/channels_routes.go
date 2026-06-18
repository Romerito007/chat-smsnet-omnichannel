package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/policy"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerChannelRoutes mounts channel connection management (authenticated,
// channel.manage) and the public, signature-authenticated inbound endpoints.
func registerChannelRoutes(r chi.Router, c *container.Container) {
	ctl := factories.ConnectionController(c)
	inbound := factories.InboundController(c)

	// Connection management (CRUD + test).
	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))
		p.Use(middleware.RequirePermission(authz.ChannelManage))

		p.Route("/channels", func(ch chi.Router) {
			ch.Get("/", ctl.List)
			ch.Post("/", ctl.Create)
			ch.Get("/{id}", ctl.Get)
			ch.Patch("/{id}", ctl.Update)
			ch.Delete("/{id}", ctl.Delete)
			ch.Post("/{id}/test", ctl.Test)
			ch.Post("/{id}/rotate-inbound-token", ctl.RotateInboundToken)
			ch.Post("/{id}/rotate-outbound-secret", ctl.RotateOutboundSecret)
		})
	})

	// Public inbound endpoints, authenticated by the channel integration token
	// (X-Inbound-Token / body inbound_token) — never the front's JWT. Rate limited
	// on a dedicated scope keyed by client IP so a noisy gateway can't exhaust the
	// shared API budget.
	r.Group(func(pub chi.Router) {
		pub.Use(middleware.RateLimitScoped(c.Redis, policy.InboundChannelRateLimit, "inbound_channel"))
		pub.Post("/inbound/channel/{channel}/messages", inbound.HandleMessage)
		pub.Post("/inbound/channel/{channel}/delivery-receipts", inbound.HandleDeliveryReceipts)
		pub.Post("/inbound/channel/{channel}/contact-identity", inbound.HandleContactIdentity)
		pub.Post("/inbound/channel/{channel}/groups", inbound.HandleGroups)
		pub.Put("/inbound/channel/{channel}/templates", inbound.HandleTemplates)
	})
}

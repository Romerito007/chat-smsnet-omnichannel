package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
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
		})
	})

	// Public inbound endpoints, authenticated by the channel signature/token.
	r.Post("/inbound/channel/{channel}/messages", inbound.HandleMessage)
	r.Post("/inbound/channel/{channel}/delivery-receipts", inbound.HandleDeliveryReceipts)
}

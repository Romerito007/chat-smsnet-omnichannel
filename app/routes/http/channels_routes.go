package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerChannelRoutes mounts channel integration management (authenticated,
// channel.manage) and the public, signature-authenticated inbound endpoint.
func registerChannelRoutes(r chi.Router, c *container.Container) {
	ctl := factories.ChannelController(c)
	inbound := factories.InboundController(c)

	// Integration management.
	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))
		p.Use(middleware.RequirePermission(authz.ChannelManage))
		p.Get("/channels", ctl.List)
		p.Post("/channels", ctl.Create)
	})

	// Inbound: public endpoint, authenticated by integration signature/secret.
	r.Post("/inbound/channel/{channel}/messages", inbound.Handle)
}

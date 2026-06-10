package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerWebhookRoutes mounts the webhook management and delivery-history
// endpoints. Every route requires webhook.manage; the tenant is derived from the
// access token.
func registerWebhookRoutes(r chi.Router, c *container.Container) {
	ctl := factories.WebhookController(c)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))
		p.Use(middleware.RequirePermission(authz.WebhookManage))

		p.Route("/webhooks", func(wh chi.Router) {
			wh.Get("/", ctl.List)
			wh.Post("/", ctl.Create)
			wh.Get("/{id}", ctl.Get)
			wh.Patch("/{id}", ctl.Update)
			wh.Delete("/{id}", ctl.Delete)
			wh.Post("/{id}/test", ctl.Test)
			wh.Get("/{id}/deliveries", ctl.Deliveries)
		})
	})
}

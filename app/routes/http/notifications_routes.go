package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerNotificationRoutes mounts the personal notification inbox and
// preferences. Every endpoint operates on the authenticated user's own
// notifications (no extra permission — notifications are personal).
func registerNotificationRoutes(r chi.Router, c *container.Container) {
	ctl := factories.NotificationController(c)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))

		p.Route("/notifications", func(n chi.Router) {
			n.Get("/", ctl.List)
			n.Post("/read-all", ctl.MarkAllRead)
			n.Post("/{id}/read", ctl.MarkRead)
			n.Get("/preferences", ctl.GetPreferences)
			n.Patch("/preferences", ctl.UpdatePreferences)
		})
	})
}

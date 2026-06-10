package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerQueueRoutes mounts queue CRUD, gated on queue.manage.
func registerQueueRoutes(r chi.Router, c *container.Container) {
	ctl := factories.QueueController(c)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))
		p.Use(middleware.RequirePermission(authz.QueueManage))

		p.Route("/queues", func(q chi.Router) {
			q.Get("/", ctl.List)
			q.Post("/", ctl.Create)
			q.Get("/{id}", ctl.Get)
			q.Patch("/{id}", ctl.Update)
			q.Delete("/{id}", ctl.Delete)
		})
	})
}

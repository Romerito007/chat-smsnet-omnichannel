package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerQueueRoutes mounts queue endpoints: reads (list/get) are available to
// any authenticated tenant user that can handle conversations (conversation.read),
// since agents need the queue list for filters and assignment; writes require
// queue.manage.
func registerQueueRoutes(r chi.Router, c *container.Container) {
	ctl := factories.QueueController(c)

	read := middleware.RequirePermission(authz.ConversationRead)
	manage := middleware.RequirePermission(authz.QueueManage)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))

		p.Route("/queues", func(q chi.Router) {
			q.With(read, catalogCache).Get("/", ctl.List)
			q.With(read).Get("/{id}", ctl.Get)
			q.With(manage).Post("/", ctl.Create)
			q.With(manage).Patch("/{id}", ctl.Update)
			q.With(manage).Delete("/{id}", ctl.Delete)
		})
	})
}

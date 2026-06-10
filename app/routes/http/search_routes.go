package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerSearchRoutes mounts the search endpoints. They require conversation.read
// (a handling agent); the service further restricts every result to the actor's
// visibility scope.
func registerSearchRoutes(r chi.Router, c *container.Container) {
	ctl := factories.SearchController(c)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))
		p.Use(middleware.RequirePermission(authz.ConversationRead))

		p.Get("/search/conversations", ctl.Conversations)
		p.Get("/search/contacts", ctl.Contacts)
		p.Get("/search/messages", ctl.Messages)
	})
}

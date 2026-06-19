package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerConversationRoutes mounts the conversations endpoints. All require
// authentication; each action is gated on its specific permission, and the
// service additionally enforces per-agent visibility (own sectors / assigned).
func registerConversationRoutes(r chi.Router, c *container.Container) {
	ctl := factories.ConversationController(c)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))

		p.Route("/conversations", func(cv chi.Router) {
			// Read.
			cv.With(middleware.RequirePermission(authz.ConversationRead)).Get("/", ctl.List)
			// Static segment registered before /{id} so it is not captured as an id.
			cv.With(middleware.RequirePermission(authz.ConversationRead)).Get("/unread-counts", ctl.UnreadCounts)
			cv.With(middleware.RequirePermission(authz.ConversationRead)).Get("/{id}", ctl.Get)
			cv.With(middleware.RequirePermission(authz.ConversationRead)).Get("/{id}/messages", ctl.ListMessages)
			cv.With(middleware.RequirePermission(authz.ConversationRead)).Get("/{id}/events", ctl.ListEvents)

			// Create + manage.
			cv.With(middleware.RequirePermission(authz.ConversationRead)).Post("/", ctl.Create)
			cv.With(middleware.RequirePermission(authz.ConversationAssign)).Patch("/{id}", ctl.Update)

			// Messaging.
			cv.With(middleware.RequirePermission(authz.MessageSend)).Post("/{id}/messages", ctl.SendMessage)
			// Edit requires message.send (author-or-elevated enforced in the service);
			// delete requires the elevated message.delete permission.
			cv.With(middleware.RequirePermission(authz.MessageSend)).Patch("/{id}/messages/{mid}", ctl.EditMessage)
			cv.With(middleware.RequirePermission(authz.MessageDelete)).Delete("/{id}/messages/{mid}", ctl.DeleteMessage)
			cv.With(middleware.RequirePermission(authz.MessageInternalNote)).Post("/{id}/internal-notes", ctl.AddInternalNote)

			// Lifecycle.
			cv.With(middleware.RequirePermission(authz.ConversationClose)).Post("/{id}/close", ctl.Close)
			cv.With(middleware.RequirePermission(authz.ConversationClose)).Post("/{id}/reopen", ctl.Reopen)

			// Typing + read receipts (realtime). Require visibility (read).
			cv.With(middleware.RequirePermission(authz.ConversationRead)).Post("/{id}/typing/start", ctl.TypingStart)
			cv.With(middleware.RequirePermission(authz.ConversationRead)).Post("/{id}/typing/stop", ctl.TypingStop)
			cv.With(middleware.RequirePermission(authz.ConversationRead)).Post("/{id}/read", ctl.Read)
			cv.With(middleware.RequirePermission(authz.ConversationRead)).Post("/{id}/unread", ctl.Unread)
		})
	})
}

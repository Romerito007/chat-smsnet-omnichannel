package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerConversationToolsRoutes mounts the tag / canned-response / close-reason
// CRUD endpoints and the conversation tag-apply endpoint. Reads are available to
// handling agents (conversation.read); catalog writes require sector.manage. The
// tag-apply endpoint records a conversation.tagged event and publishes realtime.
func registerConversationToolsRoutes(r chi.Router, c *container.Container) {
	ctl := factories.ConversationToolsController(c)

	read := middleware.RequirePermission(authz.ConversationRead)
	manage := middleware.RequirePermission(authz.SectorManage)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))

		p.Route("/tags", func(t chi.Router) {
			t.With(read, catalogCache).Get("/", ctl.ListTags)
			t.With(read).Get("/{id}", ctl.GetTag)
			t.With(manage).Post("/", ctl.CreateTag)
			t.With(manage).Patch("/{id}", ctl.UpdateTag)
			t.With(manage).Delete("/{id}", ctl.DeleteTag)
		})

		p.Route("/canned-responses", func(cr chi.Router) {
			cr.With(read, catalogCache).Get("/", ctl.ListCanned) // ?shortcut= resolves one
			cr.With(read).Get("/{id}", ctl.GetCanned)
			cr.With(manage).Post("/", ctl.CreateCanned)
			cr.With(manage).Patch("/{id}", ctl.UpdateCanned)
			cr.With(manage).Delete("/{id}", ctl.DeleteCanned)
		})

		p.Route("/close-reasons", func(cl chi.Router) {
			cl.With(read, catalogCache).Get("/", ctl.ListCloseReasons)
			cl.With(read).Get("/{id}", ctl.GetCloseReason)
			cl.With(manage).Post("/", ctl.CreateCloseReason)
			cl.With(manage).Patch("/{id}", ctl.UpdateCloseReason)
			cl.With(manage).Delete("/{id}", ctl.DeleteCloseReason)
		})

		// Apply/remove tags on a conversation (handling agents); the service
		// enforces conversation visibility and validates tags against the catalog.
		p.Route("/conversations/{id}/tags", func(tg chi.Router) {
			tg.With(read).Post("/", ctl.ApplyTags)
		})
	})
}

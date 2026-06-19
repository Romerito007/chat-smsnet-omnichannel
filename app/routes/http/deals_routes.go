package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerDealRoutes mounts the sales-deal (Kanban card) endpoints, tenant-scoped
// (the tenant comes from the access token). Reads require deal.view (own sector /
// assigned); creating/editing/moving requires deal.manage.
func registerDealRoutes(r chi.Router, c *container.Container) {
	ctl := factories.DealController(c)
	timeline := factories.DealTimelineController(c)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))

		p.With(middleware.RequirePermission(authz.DealView)).Get("/deals", ctl.List)
		p.With(middleware.RequirePermission(authz.DealView)).Get("/deals/{id}", ctl.Get)

		p.With(middleware.RequirePermission(authz.DealManage)).Post("/deals", ctl.Create)
		p.With(middleware.RequirePermission(authz.DealManage)).Post("/deals/from-conversation", ctl.CreateFromConversation)
		p.With(middleware.RequirePermission(authz.DealManage)).Patch("/deals/{id}", ctl.Update)
		p.With(middleware.RequirePermission(authz.DealManage)).Patch("/deals/{id}/stage", ctl.MoveStage)
		p.With(middleware.RequirePermission(authz.DealManage)).Post("/deals/{id}/conversations", ctl.LinkConversation)
		p.With(middleware.RequirePermission(authz.DealManage)).Post("/deals/{id}/lost", ctl.MarkLost)
		p.With(middleware.RequirePermission(authz.DealManage)).Delete("/deals/{id}", ctl.Delete)

		// Deal timeline: read the feed (deal.view) and add a manual comment (deal.manage).
		p.With(middleware.RequirePermission(authz.DealView)).Get("/deals/{id}/timeline", timeline.Feed)
		p.With(middleware.RequirePermission(authz.DealManage)).Post("/deals/{id}/timeline/comments", timeline.Comment)

		// Deal product items (deal.manage; respects the products toggle). They are read
		// back as part of GET /deals/{id}.
		p.With(middleware.RequirePermission(authz.DealManage)).Post("/deals/{id}/items", ctl.AddItem)
		p.With(middleware.RequirePermission(authz.DealManage)).Patch("/deals/{id}/items/{itemId}", ctl.UpdateItem)
		p.With(middleware.RequirePermission(authz.DealManage)).Delete("/deals/{id}/items/{itemId}", ctl.RemoveItem)
	})
}

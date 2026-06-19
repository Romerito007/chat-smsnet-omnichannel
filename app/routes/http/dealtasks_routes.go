package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerDealTaskRoutes mounts the deal-task (seller follow-up) endpoints: the
// per-deal tasks and the consolidated "my tasks" view. Tenant-scoped (the tenant
// comes from the token). Reads require deal.view; writes require deal.manage. All
// respect the tenant's tasks toggle (crmsettings.tasks_enabled).
func registerDealTaskRoutes(r chi.Router, c *container.Container) {
	ctl := factories.DealTaskController(c)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))

		p.With(middleware.RequirePermission(authz.DealView)).Get("/deals/{id}/tasks", ctl.ListByDeal)
		p.With(middleware.RequirePermission(authz.DealManage)).Post("/deals/{id}/tasks", ctl.Create)
		p.With(middleware.RequirePermission(authz.DealManage)).Patch("/deals/{id}/tasks/{taskId}", ctl.Update)
		p.With(middleware.RequirePermission(authz.DealManage)).Post("/deals/{id}/tasks/{taskId}/complete", ctl.Complete)
		p.With(middleware.RequirePermission(authz.DealManage)).Delete("/deals/{id}/tasks/{taskId}", ctl.Delete)

		// Consolidated seller task board ("my tasks") across deals.
		p.With(middleware.RequirePermission(authz.DealView)).Get("/crm/tasks", ctl.ListMine)
	})
}

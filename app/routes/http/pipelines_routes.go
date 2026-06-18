package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerPipelineRoutes mounts the sales-pipeline (Kanban funnel) endpoints,
// tenant-scoped (the tenant comes from the access token). Reads require
// pipeline.view; configuring the funnel and its stages requires pipeline.manage.
func registerPipelineRoutes(r chi.Router, c *container.Container) {
	ctl := factories.PipelineController(c)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))

		p.With(middleware.RequirePermission(authz.PipelineView)).Get("/pipelines", ctl.List)
		p.With(middleware.RequirePermission(authz.PipelineView)).Get("/pipelines/{id}", ctl.Get)

		p.With(middleware.RequirePermission(authz.PipelineManage)).Post("/pipelines", ctl.Create)
		p.With(middleware.RequirePermission(authz.PipelineManage)).Patch("/pipelines/{id}", ctl.Update)
		p.With(middleware.RequirePermission(authz.PipelineManage)).Delete("/pipelines/{id}", ctl.Delete)

		p.With(middleware.RequirePermission(authz.PipelineManage)).Post("/pipelines/{id}/stages", ctl.AddStage)
		p.With(middleware.RequirePermission(authz.PipelineManage)).Post("/pipelines/{id}/stages/reorder", ctl.ReorderStages)
		p.With(middleware.RequirePermission(authz.PipelineManage)).Patch("/pipelines/{id}/stages/{stageId}", ctl.UpdateStage)
		p.With(middleware.RequirePermission(authz.PipelineManage)).Delete("/pipelines/{id}/stages/{stageId}", ctl.DeleteStage)
	})
}

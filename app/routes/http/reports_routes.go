package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerReportRoutes mounts the operational report endpoints, all gated on
// report.view. Filters (period + sector/agent/channel) come from the query
// string; aggregations are tenant-scoped.
func registerReportRoutes(r chi.Router, c *container.Container) {
	ctl := factories.ReportController(c)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))
		p.Use(middleware.RequirePermission(authz.ReportView))

		p.Get("/reports/overview", ctl.Overview)
		p.Get("/reports/conversations", ctl.Conversations)
		p.Get("/reports/agents", ctl.Agents)
		p.Get("/reports/sectors", ctl.Sectors)
		p.Get("/reports/copilot", ctl.Copilot)
		p.Get("/reports/automation", ctl.Automation)
		p.Get("/reports/sla", ctl.SLA)
		p.Get("/reports/csat", ctl.CSAT)
	})

	// Report export is a stronger capability than viewing.
	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))
		p.Use(middleware.RequirePermission(authz.ReportExport))
		p.Post("/reports/export", ctl.Export)
	})

	// Public download: the signed, expiring token in the URL is the credential.
	r.Get("/reports/downloads/{token}", ctl.Download)
}

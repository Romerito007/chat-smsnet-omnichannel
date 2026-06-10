package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerMonitoringRoutes mounts the monitoring config endpoints. The
// on-demand, by-conversation queries (monitoring-summary, monitoring-incidents)
// live in registerExternalRoutes (a single shared /conversations/{id}/external
// subrouter) and require contact.view_connection_status.
func registerMonitoringRoutes(r chi.Router, c *container.Container) {
	ctl := factories.MonitoringController(c)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))

		// Config management.
		p.With(middleware.RequirePermission(authz.IntegrationRead)).Get("/monitoring/config", ctl.GetConfig)
		p.With(middleware.RequirePermission(authz.IntegrationConfigure)).Patch("/monitoring/config", ctl.SaveConfig)
		p.With(middleware.RequirePermission(authz.IntegrationConfigure)).Post("/monitoring/config/test", ctl.TestConfig)
	})
}

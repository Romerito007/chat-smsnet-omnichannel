package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerProviderHubRoutes mounts the providerhub config endpoints. The
// on-demand, by-conversation queries live in registerExternalRoutes (a single
// shared /conversations/{id}/external subrouter).
func registerProviderHubRoutes(r chi.Router, c *container.Container) {
	ctl := factories.ProviderHubController(c)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))

		// Static ISP catalog (slugs/labels/credential fields/actions) — quasi-static,
		// so it carries an ETag for a cheap 304 on re-fetch.
		p.With(middleware.RequirePermission(authz.IntegrationRead), catalogCache).Get("/providerhub/catalog", ctl.Catalog)

		// Config management.
		p.With(middleware.RequirePermission(authz.IntegrationRead)).Get("/providerhub/config", ctl.GetConfig)
		p.With(middleware.RequirePermission(authz.IntegrationConfigure)).Post("/providerhub/config", ctl.CreateConfig)
		p.With(middleware.RequirePermission(authz.IntegrationConfigure)).Patch("/providerhub/config", ctl.UpdateConfig)
		p.With(middleware.RequirePermission(authz.IntegrationConfigure)).Post("/providerhub/config/test", ctl.TestConfig)
	})
}

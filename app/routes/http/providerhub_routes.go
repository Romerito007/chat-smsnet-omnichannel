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

		// Gateway status (infra/env) + ISP-profile summary.
		p.With(middleware.RequirePermission(authz.IntegrationRead)).Get("/providerhub/config", ctl.GetConfig)

		// ISP profiles (many per tenant). Read with IntegrationRead, write with
		// IntegrationConfigure.
		p.With(middleware.RequirePermission(authz.IntegrationRead)).Get("/providerhub/profiles", ctl.ListProfiles)
		p.With(middleware.RequirePermission(authz.IntegrationConfigure)).Post("/providerhub/profiles", ctl.CreateProfile)
		p.With(middleware.RequirePermission(authz.IntegrationRead)).Get("/providerhub/profiles/{id}", ctl.GetProfile)
		p.With(middleware.RequirePermission(authz.IntegrationConfigure)).Patch("/providerhub/profiles/{id}", ctl.UpdateProfile)
		p.With(middleware.RequirePermission(authz.IntegrationConfigure)).Delete("/providerhub/profiles/{id}", ctl.DeleteProfile)
		p.With(middleware.RequirePermission(authz.IntegrationConfigure)).Post("/providerhub/profiles/{id}/default", ctl.SetDefaultProfile)
		p.With(middleware.RequirePermission(authz.IntegrationConfigure)).Post("/providerhub/profiles/{id}/test", ctl.TestProfile)
	})
}

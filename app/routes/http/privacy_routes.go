package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerPrivacyRoutes mounts the privacy (LGPD) endpoints. The management
// surface is gated on privacy.manage (owner/admin); the audit-log query on
// audit.view. The signed-URL download is public — the unguessable, expiring,
// HMAC-signed token is the only credential.
func registerPrivacyRoutes(r chi.Router, c *container.Container) {
	ctl := factories.PrivacyController(c)
	audit := factories.AuditController(c)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))

		p.With(middleware.RequirePermission(authz.PrivacyManage)).Group(func(m chi.Router) {
			m.Post("/privacy/contacts/{id}/export", ctl.Export)
			m.Delete("/privacy/contacts/{id}", ctl.Erase)
			m.Get("/privacy/exports/{id}", ctl.GetExport)
			m.Get("/privacy/retention", ctl.GetRetention)
			m.Patch("/privacy/retention", ctl.UpdateRetention)
		})

		p.With(middleware.RequirePermission(authz.AuditView)).Get("/audit", audit.List)
	})

	// Public: temporary signed download link for an assembled export.
	r.Get("/privacy/downloads/{token}", ctl.Download)
}

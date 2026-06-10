package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerCSATRoutes mounts CSAT survey CRUD (sector.manage), the responses
// listing (report.view) and the PUBLIC token answer (no auth — the token is the
// only credential and never exposes the conversation).
func registerCSATRoutes(r chi.Router, c *container.Container) {
	ctl := factories.CSATController(c)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))

		p.Route("/csat/surveys", func(s chi.Router) {
			manage := middleware.RequirePermission(authz.SectorManage)
			s.With(manage).Get("/", ctl.ListSurveys)
			s.With(manage).Post("/", ctl.CreateSurvey)
			s.With(manage).Get("/{id}", ctl.GetSurvey)
			s.With(manage).Patch("/{id}", ctl.UpdateSurvey)
			s.With(manage).Delete("/{id}", ctl.DeleteSurvey)
		})

		p.With(middleware.RequirePermission(authz.ReportView)).Get("/csat/responses", ctl.ListResponses)
	})

	// Public: the customer answers with just the token.
	r.Post("/csat/responses/{token}", ctl.Submit)
}

package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerAuthRoutes mounts the auth endpoints. login/refresh are public;
// logout and /me require a valid access token.
func registerAuthRoutes(r chi.Router, c *container.Container) {
	ctl := factories.AuthController(c)

	r.Post("/auth/login", ctl.Login)
	r.Post("/auth/refresh", ctl.Refresh)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))
		p.Post("/auth/logout", ctl.Logout)
		p.Get("/me", ctl.Me)
	})
}

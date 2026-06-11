package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerIAMRoutes mounts the users and roles CRUD. Both require authentication
// and the user.manage permission.
func registerIAMRoutes(r chi.Router, c *container.Container) {
	users := factories.UserController(c)
	roles := factories.RoleController(c)
	account := factories.AccountController(c)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))
		p.Use(middleware.RequirePermission(authz.UserManage))

		p.Route("/users", func(u chi.Router) {
			u.Get("/", users.List)
			u.Post("/", users.Create)
			u.Post("/invite", account.Invite)
			u.Get("/{id}", users.Get)
			u.Patch("/{id}", users.Update)
			u.Delete("/{id}", users.Delete)
		})

		p.Route("/roles", func(ro chi.Router) {
			ro.Get("/", roles.List)
			ro.Post("/", roles.Create)
			ro.Get("/{id}", roles.Get)
			ro.Patch("/{id}", roles.Update)
			ro.Delete("/{id}", roles.Delete)
		})
	})
}

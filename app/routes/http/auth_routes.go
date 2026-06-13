package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/policy"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerAuthRoutes mounts the auth endpoints. login/refresh and the account
// flows (signup, verify, invite acceptance, password reset) are public; logout,
// /me and the profile endpoints require a valid access token.
func registerAuthRoutes(r chi.Router, c *container.Container) {
	ctl := factories.AuthController(c)
	account := factories.AccountController(c)

	r.Post("/auth/login", ctl.Login)
	r.Post("/auth/refresh", ctl.Refresh)

	// Token-redeeming public endpoints (the token itself is the credential).
	r.Post("/auth/verify-email", account.VerifyEmail)
	r.Post("/auth/reset-password", account.ResetPassword)
	r.Post("/auth/accept-invite", account.AcceptInvite)

	// Abuse-prone public endpoints get a tighter per-IP rate limit on top of the
	// baseline API limit.
	r.Group(func(p chi.Router) {
		p.Use(middleware.RateLimitScoped(c.Redis, policy.SensitiveAuthRateLimit, "auth_sensitive"))
		p.Post("/auth/signup", account.Signup)
		p.Post("/auth/forgot-password", account.ForgotPassword)
		p.Post("/auth/resend-verification", account.ResendVerification)
	})

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))
		p.Post("/auth/logout", ctl.Logout)
		p.With(catalogCache).Get("/me", ctl.Me)
		p.Patch("/me", account.UpdateMe)
		p.Post("/me/change-password", account.ChangePassword)
	})
}

package middleware

import (
	"net/http"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
)

// RequirePermission gates a route on a specific permission. It must run after
// AuthContext. A missing AuthContext yields 401; a missing permission yields 403.
func RequirePermission(p authz.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ac, ok := authz.FromContext(r.Context())
			if !ok {
				WriteError(w, r, apperror.Unauthorized("authentication required"))
				return
			}
			if !ac.Has(p) {
				WriteError(w, r, apperror.Forbidden("missing permission: "+string(p)))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

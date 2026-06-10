package middleware

import (
	"net/http"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/auth"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// HeaderAuthorization carries the bearer access token.
const HeaderAuthorization = "Authorization"

// AuthContext authenticates the request from its bearer access token and
// populates the context with the resolved AuthContext and tenant scope.
//
// The tenant is taken exclusively from the signed token — never from a client
// header — which is the core multi-tenant security invariant. Requests without a
// valid token are rejected with 401.
func AuthContext(tm auth.TokenManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := bearerToken(r)
			if token == "" {
				WriteError(w, r, apperror.Unauthorized("missing bearer token"))
				return
			}
			claims, err := tm.VerifyAccess(token)
			if err != nil {
				WriteError(w, r, apperror.Unauthorized("invalid or expired token"))
				return
			}

			ac := authz.NewAuthContext(
				claims.TenantID,
				claims.UserID,
				claims.Permissions,
				claims.SectorIDs,
				claims.SectorScope,
			)
			ctx := authz.WithAuthContext(r.Context(), ac)
			ctx = shared.WithTenant(ctx, claims.TenantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get(HeaderAuthorization)
	const prefix = "Bearer "
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}

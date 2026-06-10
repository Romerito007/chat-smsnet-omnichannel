package middleware

import (
	"net/http"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// HeaderTenantID carries the tenant scope on inbound requests until real auth
// derives it from the authenticated principal.
const HeaderTenantID = "X-Tenant-Id"

// TenantContext extracts the tenant id and (once auth is wired) the actor, and
// stores them on the request context so repositories can enforce tenant scope.
// In the foundation it trusts the X-Tenant-Id header; the auth domain will
// later replace this with a verified claim.
func TenantContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if tenant := r.Header.Get(HeaderTenantID); tenant != "" {
			ctx = shared.WithTenant(ctx, tenant)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

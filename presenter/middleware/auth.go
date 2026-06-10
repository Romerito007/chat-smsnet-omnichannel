package middleware

import (
	"net/http"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// HeaderAuthorization and HeaderAPIKey carry credentials.
const (
	HeaderAuthorization = "Authorization"
	HeaderAPIKey        = "X-Api-Key"
)

// AuthStub is a placeholder authentication middleware. It does NOT verify
// credentials — it only shapes the request context the way real auth will, so
// downstream middleware/handlers can be developed against a stable contract.
//
// Behavior (to be replaced by the `auth` domain / JWT verification):
//   - If a Bearer token or X-Api-Key is present, it derives a stub Actor and,
//     when an X-Tenant-Id is also present, the tenant scope.
//   - It never rejects: protected routes will gate on real auth + authz later.
func AuthStub(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		token := bearerToken(r)
		apiKey := r.Header.Get(HeaderAPIKey)
		if token == "" && apiKey == "" {
			next.ServeHTTP(w, r)
			return
		}

		tenant, _ := shared.TenantFrom(ctx)
		actor := shared.Actor{
			UserID:   "stub-user",
			TenantID: tenant,
			Roles:    []string{"owner"},
		}
		ctx = shared.WithActor(ctx, actor)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get(HeaderAuthorization)
	if h == "" {
		return ""
	}
	const prefix = "Bearer "
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}

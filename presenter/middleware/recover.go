package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// Recover turns a panic in any handler into a 500 error envelope instead of
// dropping the connection, logging the stack for diagnosis.
func Recover(logger shared.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					shared.LoggerFrom(r.Context(), logger).Error("panic recovered",
						"panic", rec,
						"stack", string(debug.Stack()),
					)
					WriteError(w, r, apperror.Internal("internal server error"))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

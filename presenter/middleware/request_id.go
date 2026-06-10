package middleware

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// HeaderRequestID is the canonical request-id header.
const HeaderRequestID = "X-Request-Id"

// RequestID ensures every request carries a request id (honoring an inbound one
// or generating a new one), stores it on the context and echoes it back. It
// also logs method, path, status and duration — the observability baseline
// required by the architecture.
func RequestID(logger shared.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rid := r.Header.Get(HeaderRequestID)
			if rid == "" {
				rid = uuid.NewString()
			}
			ctx := shared.WithRequestID(r.Context(), rid)
			w.Header().Set(HeaderRequestID, rid)

			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			start := time.Now()
			next.ServeHTTP(sw, r.WithContext(ctx))

			shared.LoggerFrom(ctx, logger).Info("http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", sw.status,
				"duration_ms", time.Since(start).Milliseconds(),
			)
		})
	}
}

// statusWriter captures the response status code for logging.
type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.status = code
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

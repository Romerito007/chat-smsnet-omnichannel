package middleware

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Telemetry instruments handlers with OpenTelemetry: it starts a server span per
// request and records the standard HTTP server metrics (request count, duration,
// in-flight) against the global providers installed by app/providers. When OTEL
// is disabled the global providers are no-ops, so this stays effectively free.
func Telemetry(operation string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(next, operation)
	}
}

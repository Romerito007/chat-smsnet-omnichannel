// Package middleware holds the HTTP/WS border middlewares and the shared
// request/response helpers used by every controller.
package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// errorEnvelope is the documented error response body.
type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code      apperror.Code  `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
	RequestID string         `json:"request_id,omitempty"`
}

// WriteJSON serializes v as JSON with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// WriteError normalizes any error into an AppError and renders the standard
// envelope, attaching the request id from the context.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	appErr := apperror.From(err)
	// A server-side failure (5xx) carries a real cause that must NOT reach the
	// client (the body keeps the generic message); log it server-side, enriched
	// with request_id/tenant_id, so the actual error is diagnosable. Known causes
	// (e.g. a duplicate key) are already mapped to 4xx by apperror/MapError and so
	// are not logged here as internal failures.
	if appErr.HTTPStatus() >= 500 {
		shared.LoggerFrom(r.Context(), slog.Default()).Error("request failed",
			"code", string(appErr.Code), "error", appErr.Error())
	}
	WriteJSON(w, appErr.HTTPStatus(), errorEnvelope{
		Error: errorBody{
			Code:      appErr.Code,
			Message:   appErr.Message,
			Details:   appErr.Details,
			RequestID: shared.RequestIDFrom(r.Context()),
		},
	})
}

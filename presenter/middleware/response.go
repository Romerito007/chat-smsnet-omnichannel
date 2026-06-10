// Package middleware holds the HTTP/WS border middlewares and the shared
// request/response helpers used by every controller.
package middleware

import (
	"encoding/json"
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
	WriteJSON(w, appErr.HTTPStatus(), errorEnvelope{
		Error: errorBody{
			Code:      appErr.Code,
			Message:   appErr.Message,
			Details:   appErr.Details,
			RequestID: shared.RequestIDFrom(r.Context()),
		},
	})
}

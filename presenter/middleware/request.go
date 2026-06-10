package middleware

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// DecodeJSON decodes the request body into dst, rejecting unknown fields and
// returning a validation AppError on malformed input.
func DecodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return apperror.Validation("request body is required")
		}
		return apperror.Validation("invalid JSON body").Wrap(err)
	}
	return nil
}

// PageFromRequest parses keyset pagination params (limit, cursor) from the query.
func PageFromRequest(r *http.Request) shared.PageRequest {
	q := r.URL.Query()
	page := shared.PageRequest{Cursor: q.Get("cursor")}
	if l := q.Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			page.Limit = n
		}
	}
	return page.Normalize()
}

// ClientIP extracts a best-effort client IP for audit/session records.
func ClientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return fwd
	}
	return r.RemoteAddr
}

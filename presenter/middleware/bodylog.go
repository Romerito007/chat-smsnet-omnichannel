package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// maxLoggedBody bounds the request/response bytes captured for logging, so an
// attachment or base64 blob never explodes the log.
const maxLoggedBody = 4 << 10 // 4 KiB

// readAndRestoreBody reads up to maxLoggedBody+1 bytes for logging and restores
// r.Body so the handler still sees the full payload.
func readAndRestoreBody(r *http.Request) []byte {
	if r.Body == nil {
		return nil
	}
	// Read the whole body (capped by the route's own limits) and restore it.
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return nil
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
	return raw
}

// loggableBody returns a redacted, truncated string of the request body suitable
// for logs. JSON is parsed and sensitive keys are redacted; non-JSON is truncated
// as-is; multipart bodies are omitted (binary/huge).
func loggableBody(contentType string, raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	ct := strings.ToLower(contentType)
	if strings.HasPrefix(ct, "multipart/form-data") {
		return "[multipart omitted]"
	}
	if strings.HasPrefix(ct, "application/json") {
		var v any
		if err := json.Unmarshal(raw, &v); err == nil {
			redactJSON(v)
			if b, err := json.Marshal(v); err == nil {
				return truncateStr(string(b), maxLoggedBody)
			}
		}
	}
	return truncateStr(string(raw), maxLoggedBody)
}

// loggableQuery returns the request query string with sensitive parameter values
// redacted, so e.g. ?token=… never appears in clear in the logs.
func loggableQuery(values url.Values) string {
	if len(values) == 0 {
		return ""
	}
	redacted := make(url.Values, len(values))
	for k, vs := range values {
		if isSensitiveKey(k) {
			redacted[k] = []string{"[REDACTED]"}
			continue
		}
		redacted[k] = vs
	}
	return truncateStr(redacted.Encode(), maxLoggedBody)
}

// redactJSON walks a decoded JSON value and replaces sensitive field values with
// "[REDACTED]" in place. Matching is by key (case-insensitive).
func redactJSON(v any) {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if isSensitiveKey(k) {
				t[k] = "[REDACTED]"
				continue
			}
			redactJSON(val)
		}
	case []any:
		for _, e := range t {
			redactJSON(e)
		}
	}
}

// isSensitiveKey reports whether a JSON key holds a credential that must never be
// logged in clear, even with body logging enabled.
func isSensitiveKey(k string) bool {
	lk := strings.ToLower(strings.TrimSpace(k))
	switch lk {
	case "password", "current_password", "new_password", "api_key", "secret",
		"inbound_token", "outbound_secret", "authorization", "token", "refresh_token",
		"access_token":
		return true
	}
	return strings.Contains(lk, "password") || strings.HasSuffix(lk, "_token") ||
		strings.HasSuffix(lk, "_secret") || strings.HasSuffix(lk, "_key")
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…[truncated]"
}

// loggedError is the minimal shape parsed from a captured error response so the
// log shows error.code/message (never any sensitive body — errors don't carry it).
type loggedError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func parseErrorEnvelope(raw []byte) (code, message string) {
	var e loggedError
	if json.Unmarshal(raw, &e) == nil {
		return e.Error.Code, e.Error.Message
	}
	return "", ""
}

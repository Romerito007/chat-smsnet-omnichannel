package middleware

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// HeaderRequestID is the canonical request-id header.
const HeaderRequestID = "X-Request-Id"

// RequestID ensures every request carries a request id (honoring an inbound one
// or generating a new one), stores it on the context and echoes it back. It
// also logs method, path, status and duration — the observability baseline.
// When logBody is true (LOG_REQUEST_BODY, dev-only), it additionally logs the
// redacted+truncated request body and, for error responses, error.code/message —
// credentials are always redacted regardless of the flag.
func RequestID(logger shared.Logger, logBody bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rid := r.Header.Get(HeaderRequestID)
			if rid == "" {
				rid = uuid.NewString()
			}
			ctx := shared.WithRequestID(r.Context(), rid)
			// Capture client ip + user-agent so the audit recorder can attach them
			// to entries produced deep in the service layer.
			ctx = shared.WithAuditMeta(ctx, ClientIP(r), r.UserAgent())
			w.Header().Set(HeaderRequestID, rid)

			var reqBody []byte
			if logBody {
				reqBody = readAndRestoreBody(r)
			}

			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			if logBody {
				sw.capture = &bytes.Buffer{} // capped in Write
			}
			start := time.Now()
			next.ServeHTTP(sw, r.WithContext(ctx))

			fields := []any{
				"method", r.Method,
				"path", r.URL.Path,
				"status", sw.status,
				"duration_ms", time.Since(start).Milliseconds(),
			}
			if logBody {
				if q := loggableQuery(r.URL.Query()); q != "" {
					fields = append(fields, "query", q)
				}
				if b := loggableBody(r.Header.Get("Content-Type"), reqBody); b != "" {
					fields = append(fields, "body", b)
				}
				if sw.status >= 400 {
					if code, msg := parseErrorEnvelope(sw.capture.Bytes()); code != "" {
						fields = append(fields, "error_code", code, "error_message", msg)
					}
				}
			}
			shared.LoggerFrom(ctx, logger).Info("http request", fields...)
		})
	}
}

// statusWriter captures the response status code (and, when capture != nil, a
// capped copy of the body) for logging.
type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	capture     *bytes.Buffer
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
	if w.capture != nil && w.capture.Len() < maxLoggedBody {
		w.capture.Write(b[:min(len(b), maxLoggedBody-w.capture.Len())])
	}
	return w.ResponseWriter.Write(b)
}

// Hijack lets WebSocket upgrades work through this wrapper by delegating to the
// underlying ResponseWriter when it supports hijacking.
func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, fmt.Errorf("underlying ResponseWriter does not support hijacking")
}

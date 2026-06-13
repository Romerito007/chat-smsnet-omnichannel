package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ConditionalCache adds HTTP conditional caching (a strong, content-derived ETag
// + a short private Cache-Control) to GET responses, so a client re-navigating a
// quasi-static catalog (tags/sectors/queues/close-reasons/canned-responses, /me)
// gets a cheap 304 Not Modified instead of refetching the full body.
//
// The ETag is a hash of the serialized response body, so it is inherently
// tenant/user-scoped: a different tenant produces a different body and therefore
// a different ETag (and two tenants with byte-identical bodies legitimately share
// a 304 — no body is ever disclosed on a 304). Apply ONLY to stable reads, never
// to volatile lists like /conversations.
//
// Note: this is HTTP conditional caching only. A server-side read-cache (e.g.
// Redis with write invalidation) for these catalogs is a possible future step
// and is intentionally out of scope here.
func ConditionalCache(maxAge time.Duration) func(http.Handler) http.Handler {
	cacheControl := fmt.Sprintf("private, max-age=%d", int(maxAge.Seconds()))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only safe, cacheable reads carry validators.
			if r.Method != http.MethodGet {
				next.ServeHTTP(w, r)
				return
			}
			cw := &cacheWriter{ResponseWriter: w, buf: &bytes.Buffer{}, status: http.StatusOK}
			next.ServeHTTP(cw, r)

			// Only a successful response is validated/cached; everything else is
			// passed through unchanged (e.g. a 403/500 must not be cached).
			if cw.status != http.StatusOK {
				cw.flush()
				return
			}
			sum := sha256.Sum256(cw.buf.Bytes())
			etag := `"` + hex.EncodeToString(sum[:16]) + `"` // strong, quoted
			h := w.Header()
			h.Set("ETag", etag)
			h.Set("Cache-Control", cacheControl)
			if etagMatches(r.Header.Get("If-None-Match"), etag) {
				w.WriteHeader(http.StatusNotModified) // 304, no body
				return
			}
			cw.flush()
		})
	}
}

// etagMatches reports whether the If-None-Match header lists etag (or "*").
func etagMatches(header, etag string) bool {
	if header == "" {
		return false
	}
	for _, tok := range strings.Split(header, ",") {
		if t := strings.TrimSpace(tok); t == "*" || t == etag {
			return true
		}
	}
	return false
}

// cacheWriter buffers the response so its body can be hashed and conditionally
// replaced by a 304. Headers (e.g. Content-Type set by WriteJSON) still go to the
// underlying writer; only the status and body are deferred until flush.
type cacheWriter struct {
	http.ResponseWriter
	buf         *bytes.Buffer
	status      int
	wroteHeader bool
	flushed     bool
}

func (w *cacheWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.status = code
		w.wroteHeader = true
	}
}

func (w *cacheWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.buf.Write(b)
}

// flush writes the captured status and buffered body to the underlying writer.
func (w *cacheWriter) flush() {
	if w.flushed {
		return
	}
	w.flushed = true
	w.ResponseWriter.WriteHeader(w.status)
	_, _ = w.ResponseWriter.Write(w.buf.Bytes())
}

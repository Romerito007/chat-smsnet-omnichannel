package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/redis"
)

// HeaderIdempotencyKey carries the client-supplied idempotency key.
const HeaderIdempotencyKey = "Idempotency-Key"

// idempotencyTTL bounds how long a stored response is replayed.
const idempotencyTTL = 24 * time.Hour

// storedResponse is the cached outcome of a previously processed request.
type storedResponse struct {
	PayloadHash string      `json:"payload_hash"`
	Status      int         `json:"status"`
	Body        []byte      `json:"body"`
	Headers     http.Header `json:"headers"`
}

// Idempotency makes resource-creating POSTs safe to retry. When an
// Idempotency-Key is present, the first response is stored in Redis keyed by
// (tenant, key); subsequent calls with the same key replay it. A different
// payload under the same key is a conflict.
func Idempotency(rdb redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get(HeaderIdempotencyKey)
			if key == "" || r.Method != http.MethodPost {
				next.ServeHTTP(w, r)
				return
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				WriteError(w, r, apperror.Validation("unreadable request body"))
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			payloadHash := hashPayload(body)

			redisKey := idempotencyKey(r.Context(), key)
			if replayed, ok := tryReplay(r.Context(), rdb, redisKey, payloadHash, w, r); ok {
				_ = replayed
				return
			}

			rec := &recordingWriter{ResponseWriter: w, header: http.Header{}, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			// Only persist successful creations.
			if rec.status >= 200 && rec.status < 300 {
				store := storedResponse{
					PayloadHash: payloadHash,
					Status:      rec.status,
					Body:        rec.body.Bytes(),
					Headers:     rec.header,
				}
				if raw, err := json.Marshal(store); err == nil {
					_ = rdb.Set(r.Context(), redisKey, raw, idempotencyTTL).Err()
				}
			}
		})
	}
}

// tryReplay returns (handled, true) when a stored response was replayed or a
// conflict was emitted.
func tryReplay(ctx context.Context, rdb redis.Client, redisKey, payloadHash string, w http.ResponseWriter, r *http.Request) (bool, bool) {
	raw, err := rdb.Get(ctx, redisKey).Bytes()
	if err != nil {
		return false, false // cache miss or Redis down: process normally
	}
	var stored storedResponse
	if json.Unmarshal(raw, &stored) != nil {
		return false, false
	}
	if stored.PayloadHash != payloadHash {
		WriteError(w, r, apperror.Conflict("idempotency key reused with a different payload"))
		return true, true
	}
	for k, vals := range stored.Headers {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	w.Header().Set("Idempotent-Replay", "true")
	w.WriteHeader(stored.Status)
	_, _ = w.Write(stored.Body)
	return true, true
}

func idempotencyKey(ctx context.Context, key string) string {
	tenant, _ := shared.TenantFrom(ctx)
	if tenant == "" {
		tenant = "anon"
	}
	return "idem:" + tenant + ":" + key
}

func hashPayload(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// recordingWriter captures status, headers and body so a successful response can
// be cached for replay.
type recordingWriter struct {
	http.ResponseWriter
	header      http.Header
	status      int
	body        bytes.Buffer
	wroteHeader bool
}

func (w *recordingWriter) Header() http.Header { return w.ResponseWriter.Header() }

func (w *recordingWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.status = code
	w.wroteHeader = true
	// Snapshot headers for the cache.
	for k, v := range w.ResponseWriter.Header() {
		w.header[k] = append([]string(nil), v...)
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *recordingWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

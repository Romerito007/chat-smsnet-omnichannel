package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/policy"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/redis"
)

// RateLimit applies a fixed-window limit per (tenant, client-ip) using Redis as
// the shared counter, so the limit holds across all API nodes.
func RateLimit(rdb redis.Client, limit policy.RateLimit) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := rateKey(r)

			count, err := rdb.Incr(r.Context(), key).Result()
			if err != nil {
				// Fail open: a Redis blip must not take down the API.
				next.ServeHTTP(w, r)
				return
			}
			if count == 1 {
				_ = rdb.Expire(r.Context(), key, limit.Window).Err()
			}
			if count > int64(limit.Requests) {
				w.Header().Set("Retry-After", strconv.Itoa(int(limit.Window/time.Second)))
				WriteError(w, r, apperror.RateLimited("rate limit exceeded"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func rateKey(r *http.Request) string {
	tenant, _ := shared.TenantFrom(r.Context())
	if tenant == "" {
		tenant = "anon"
	}
	return "ratelimit:" + tenant + ":" + clientIP(r)
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return fwd
	}
	return r.RemoteAddr
}

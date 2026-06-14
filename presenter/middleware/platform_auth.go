package middleware

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
)

// HeaderPlatformKey carries the platform service credential, formatted
// "key_id.secret". The platform plane is ABOVE tenant isolation, so this header
// is validated by PlatformAuth — never by AuthContext — and it never populates a
// tenant context.
const HeaderPlatformKey = "X-Platform-Key"

type platformKeyCtxKey struct{}

// PlatformAuth authenticates a request from its X-Platform-Key against the
// configured key set (key id -> SHA-256 hex of the secret). On success it stores
// the matched key id in the context (for audit) and sets NO tenant and NO
// permissions. Requests without a valid platform key are rejected with 401.
//
// An empty key set disables the platform plane entirely (every request is 401),
// so the provisioning endpoint is inert unless keys are configured.
func PlatformAuth(keys map[string]string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			keyID, ok := verifyPlatformKey(keys, r.Header.Get(HeaderPlatformKey))
			if !ok {
				WriteError(w, r, apperror.Unauthorized("invalid platform key"))
				return
			}
			ctx := context.WithValue(r.Context(), platformKeyCtxKey{}, keyID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// verifyPlatformKey splits "key_id.secret", looks up the key id and compares the
// SHA-256 of the presented secret to the stored hash in constant time.
func verifyPlatformKey(keys map[string]string, header string) (string, bool) {
	if len(keys) == 0 || header == "" {
		return "", false
	}
	keyID, secret, ok := strings.Cut(strings.TrimSpace(header), ".")
	if !ok || keyID == "" || secret == "" {
		return "", false
	}
	want, ok := keys[keyID]
	if !ok {
		return "", false
	}
	sum := sha256.Sum256([]byte(secret))
	got := hex.EncodeToString(sum[:])
	if subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
		return "", false
	}
	return keyID, true
}

// PlatformKeyID returns the platform key id that authorized the request, or "".
func PlatformKeyID(ctx context.Context) string {
	if v, ok := ctx.Value(platformKeyCtxKey{}).(string); ok {
		return v
	}
	return ""
}

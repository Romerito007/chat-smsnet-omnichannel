package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
)

func hashSecret(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func TestPlatformAuth(t *testing.T) {
	keys := map[string]string{"prov1": hashSecret("s3cret")}

	cases := []struct {
		name   string
		keySet map[string]string
		header string
		want   int // 200 = passed through, 401 = rejected
	}{
		{"valid key", keys, "prov1.s3cret", http.StatusOK},
		{"wrong secret", keys, "prov1.nope", http.StatusUnauthorized},
		{"unknown key id", keys, "ghost.s3cret", http.StatusUnauthorized},
		{"missing dot", keys, "prov1s3cret", http.StatusUnauthorized},
		{"empty header", keys, "", http.StatusUnauthorized},
		{"no keys configured", nil, "prov1.s3cret", http.StatusUnauthorized},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var gotKeyID string
			h := PlatformAuth(c.keySet)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotKeyID = PlatformKeyID(r.Context())
				w.WriteHeader(http.StatusOK)
			}))
			req := httptest.NewRequest(http.MethodPost, "/v1/platform/tenants", nil)
			if c.header != "" {
				req.Header.Set(HeaderPlatformKey, c.header)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != c.want {
				t.Fatalf("status = %d, want %d", rec.Code, c.want)
			}
			if c.want == http.StatusOK && gotKeyID != "prov1" {
				t.Errorf("expected key id 'prov1' in context, got %q", gotKeyID)
			}
		})
	}
}

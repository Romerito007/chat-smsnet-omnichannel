package openapi

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"sync"

	"gopkg.in/yaml.v3"
)

var (
	once     sync.Once
	jsonOnce []byte
)

// JSON returns the spec as indented JSON (cached after first build).
func JSON() []byte {
	once.Do(func() {
		b, err := json.MarshalIndent(Build(), "", "  ")
		if err != nil { // Build() is static; a marshal failure is a programmer error.
			panic("openapi: marshal json: " + err.Error())
		}
		jsonOnce = b
	})
	return jsonOnce
}

// YAML returns the spec as YAML (used to generate the committed docs/openapi.yaml).
func YAML() ([]byte, error) {
	return yaml.Marshal(Build())
}

// Config controls how GET /openapi.json is served.
type Config struct {
	// Public serves the spec without auth (intended for development). When false
	// (production), basic auth is required.
	Public bool
	// BasicUser/BasicPass gate the spec in production. If Public is false and
	// either is empty, the endpoint is locked (401) — safe by default.
	BasicUser string
	BasicPass string
}

// Handler serves the OpenAPI document as application/json. In production it is
// behind HTTP basic auth; in development it is public.
func Handler(cfg Config) http.HandlerFunc {
	spec := JSON()
	return func(w http.ResponseWriter, r *http.Request) {
		if !cfg.Public && !authorized(cfg, r) {
			w.Header().Set("WWW-Authenticate", `Basic realm="openapi"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=300")
		_, _ = w.Write(spec)
	}
}

func authorized(cfg Config, r *http.Request) bool {
	if cfg.BasicUser == "" || cfg.BasicPass == "" {
		return false // locked when unconfigured in production
	}
	u, pw, ok := r.BasicAuth()
	if !ok {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(u), []byte(cfg.BasicUser)) == 1 &&
		subtle.ConstantTimeCompare([]byte(pw), []byte(cfg.BasicPass)) == 1
}

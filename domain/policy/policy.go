// Package policy centralizes cross-cutting business policies (e.g. rate limits,
// quotas, tenant-level feature toggles). It deliberately depends only on the
// shared and authz packages so it stays at the bottom of the dependency graph.
package policy

import "time"

// RateLimit describes a token-bucket style limit applied by the ratelimit
// middleware and Redis.
type RateLimit struct {
	// Requests allowed within Window.
	Requests int
	Window   time.Duration
}

// DefaultAPIRateLimit is the baseline per-actor API limit.
var DefaultAPIRateLimit = RateLimit{Requests: 120, Window: time.Minute}

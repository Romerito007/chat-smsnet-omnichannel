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

// SensitiveAuthRateLimit is a tighter per-IP limit for the abuse-prone,
// unauthenticated account endpoints (signup, forgot-password,
// resend-verification) on top of the baseline API limit.
var SensitiveAuthRateLimit = RateLimit{Requests: 5, Window: time.Minute}

// InboundChannelRateLimit caps the public, token-authenticated channel endpoints
// (inbound messages + delivery receipts) per client IP, so an external gateway
// integrating by integration token can't exhaust the shared API budget.
var InboundChannelRateLimit = RateLimit{Requests: 600, Window: time.Minute}

// PlatformProvisionRateLimit caps the platform-plane provisioning endpoint per
// client IP. Provisioning is rare and the provisioner has a stable egress, so the
// budget is small — a strict backstop against a leaked-key flood.
var PlatformProvisionRateLimit = RateLimit{Requests: 20, Window: time.Minute}

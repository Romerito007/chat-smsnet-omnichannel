// Package contracts holds the platform-plane provisioning inputs and result. The
// platform plane sits above tenant isolation: it creates tenants and nothing else.
package contracts

import "time"

// ProvisionTenant is the input to create a tenant + its owner. ExternalRef is the
// provisioner's natural key (durable idempotency). KeyID identifies the platform
// key that authorized the call (for audit); it is set by the controller from the
// authenticated platform context, never from the body.
type ProvisionTenant struct {
	TenantName    string
	OwnerName     string
	OwnerEmail    string
	OwnerPassword string
	ExternalRef   string
	KeyID         string
}

// ProvisionResult is the outcome of a provision. AccessToken is a ready-to-use
// tenant-scoped access token for the owner (no extra login step). Created is false
// when an existing tenant was returned for a repeated ExternalRef (retry-safe).
type ProvisionResult struct {
	TenantID        string
	TenantName      string
	OwnerID         string
	OwnerEmail      string
	AccessToken     string
	AccessExpiresAt time.Time
	Created         bool
}

// Package platform holds the request/response DTOs for the platform-plane
// provisioning endpoint (create tenant + owner).
package platform

import (
	"time"

	pcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/platform/contracts"
)

// ProvisionTenantRequest is the body of POST /v1/platform/tenants. external_ref is
// the provisioner's natural key (durable idempotency on retry).
type ProvisionTenantRequest struct {
	TenantName    string `json:"tenant_name"`
	OwnerName     string `json:"owner_name"`
	OwnerEmail    string `json:"owner_email"`
	OwnerPassword string `json:"owner_password"`
	ExternalRef   string `json:"external_ref"`
}

// ToCommand maps to the service command. keyID comes from the authenticated
// platform context (never the body).
func (r ProvisionTenantRequest) ToCommand(keyID string) pcontracts.ProvisionTenant {
	return pcontracts.ProvisionTenant{
		TenantName:    r.TenantName,
		OwnerName:     r.OwnerName,
		OwnerEmail:    r.OwnerEmail,
		OwnerPassword: r.OwnerPassword,
		ExternalRef:   r.ExternalRef,
		KeyID:         keyID,
	}
}

// tenantRef / ownerRef are the minimal identifiers echoed back.
type tenantRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ownerRef struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// ProvisionTenantResponse returns the created (or existing) tenant + owner and a
// ready-to-use tenant-scoped access token (no extra login step).
type ProvisionTenantResponse struct {
	Tenant          tenantRef `json:"tenant"`
	Owner           ownerRef  `json:"owner"`
	AccessToken     string    `json:"access_token"`
	TokenType       string    `json:"token_type"`
	AccessExpiresAt time.Time `json:"access_expires_at"`
	Created         bool      `json:"created"`
}

// NewProvisionResponse maps the service result.
func NewProvisionResponse(res pcontracts.ProvisionResult) ProvisionTenantResponse {
	return ProvisionTenantResponse{
		Tenant:          tenantRef{ID: res.TenantID, Name: res.TenantName},
		Owner:           ownerRef{ID: res.OwnerID, Email: res.OwnerEmail},
		AccessToken:     res.AccessToken,
		TokenType:       "Bearer",
		AccessExpiresAt: res.AccessExpiresAt,
		Created:         res.Created,
	}
}

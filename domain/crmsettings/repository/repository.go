// Package repository declares the CRM-settings persistence contract.
package repository

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/crmsettings/entity"
)

// CRMSettingsRepository persists a tenant's CRM settings (one document per tenant,
// scope from ctx).
type CRMSettingsRepository interface {
	// Get returns the tenant's settings, or a NotFound error when none exist yet.
	Get(ctx context.Context) (*entity.CRMSettings, error)
	// Upsert creates or replaces the tenant's settings document.
	Upsert(ctx context.Context, s *entity.CRMSettings) error
}

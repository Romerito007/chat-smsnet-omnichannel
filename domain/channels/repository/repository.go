// Package repository declares the channels persistence contracts.
package repository

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// IntegrationRepository persists channel integrations.
type IntegrationRepository interface {
	Create(ctx context.Context, i *entity.Integration) error
	FindByID(ctx context.Context, id string) (*entity.Integration, error)
	List(ctx context.Context, page shared.PageRequest) ([]*entity.Integration, error)
	// FindByIntegrationKey looks an integration up by its public key. It is NOT
	// tenant-scoped: inbound requests are pre-authentication and the matched
	// record carries the authoritative tenant.
	FindByIntegrationKey(ctx context.Context, integrationKey string) (*entity.Integration, error)
}

// InboundRepository is the idempotency ledger for processed inbound messages.
type InboundRepository interface {
	// FindByExternalID returns the ledger record for (tenant, channel,
	// external_message_id), or a not_found AppError.
	FindByExternalID(ctx context.Context, channel, externalMessageID string) (*entity.InboundRecord, error)
	// Create inserts a ledger record. A duplicate (already processed) surfaces as
	// a conflict AppError, which the caller treats as idempotent success.
	Create(ctx context.Context, r *entity.InboundRecord) error
}

// Package repository declares the channels persistence contracts.
package repository

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// ConnectionRepository persists channel connections.
type ConnectionRepository interface {
	Create(ctx context.Context, c *entity.ChannelConnection) error
	Update(ctx context.Context, c *entity.ChannelConnection) error
	Delete(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (*entity.ChannelConnection, error)
	List(ctx context.Context, page shared.PageRequest) ([]*entity.ChannelConnection, error)
	// FindByInboundTokenHash resolves a connection pre-auth (inbound/receipts) by
	// the SHA-256 hash of its integration token; not tenant-scoped — the matched
	// record carries the tenant.
	FindByInboundTokenHash(ctx context.Context, tokenHash string) (*entity.ChannelConnection, error)
}

// OutboundDeliveryRepository persists outbound delivery records.
type OutboundDeliveryRepository interface {
	Create(ctx context.Context, d *entity.OutboundDelivery) error
	Update(ctx context.Context, d *entity.OutboundDelivery) error
	FindByID(ctx context.Context, id string) (*entity.OutboundDelivery, error)
	// FindByExternalMessageID locates a delivery by the channel's external id,
	// used to apply delivery receipts.
	FindByExternalMessageID(ctx context.Context, externalMessageID string) (*entity.OutboundDelivery, error)
}

// InboundRepository is the idempotency ledger for processed inbound messages.
type InboundRepository interface {
	FindByExternalID(ctx context.Context, channel, externalMessageID string) (*entity.InboundRecord, error)
	Create(ctx context.Context, r *entity.InboundRecord) error
}

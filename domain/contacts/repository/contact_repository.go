// Package repository declares the contact persistence contract.
package repository

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// ContactRepository persists contacts within a tenant (scope from context).
type ContactRepository interface {
	Create(ctx context.Context, c *entity.Contact) error
	Update(ctx context.Context, c *entity.Contact) error
	FindByID(ctx context.Context, id string) (*entity.Contact, error)
	// FindByChannelIdentity locates a contact by one of its channel identities.
	FindByChannelIdentity(ctx context.Context, channel, externalID string) (*entity.Contact, error)
	List(ctx context.Context, page shared.PageRequest) ([]*entity.Contact, error)
}

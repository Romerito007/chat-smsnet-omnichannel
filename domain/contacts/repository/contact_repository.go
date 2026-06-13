// Package repository declares the contact persistence contract.
package repository

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// ContactRepository persists contacts within a tenant (scope from context).
type ContactRepository interface {
	Create(ctx context.Context, c *entity.Contact) error
	Update(ctx context.Context, c *entity.Contact) error
	FindByID(ctx context.Context, id string) (*entity.Contact, error)
	// FindByIDs batch-loads contacts by id within the tenant (missing ids absent),
	// used to resolve contact avatars for a conversation list page without N+1.
	FindByIDs(ctx context.Context, ids []string) ([]*entity.Contact, error)
	// FindByChannelIdentity locates a contact by one of its channel identities.
	FindByChannelIdentity(ctx context.Context, channel, externalID string) (*entity.Contact, error)
	// FindByDocument / FindByPhone locate a contact for deduplication; the phone
	// match covers both the primary phone and the phones array. A not_found error
	// means there is no duplicate.
	FindByDocument(ctx context.Context, document string) (*entity.Contact, error)
	FindByPhone(ctx context.Context, phone string) (*entity.Contact, error)
	// List returns a page of contacts matching filter (free-text query plus the
	// optional name/phone substring and tag-id field filters, combined with AND).
	List(ctx context.Context, filter contracts.ListFilter, page shared.PageRequest) ([]*entity.Contact, error)
}

// Package repository declares the group persistence contract.
package repository

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/groups/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/groups/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// GroupRepository persists known WhatsApp groups within a tenant (scope from ctx).
type GroupRepository interface {
	// UpsertBatch idempotently upserts a sync batch by (tenant_id, group_jid). It
	// updates the metadata of existing groups but PRESERVES Attend (the operator's
	// choice is never reset by a sync); new groups are inserted with Attend=true.
	// Returns how many documents were inserted or modified.
	UpsertBatch(ctx context.Context, channelID string, groups []contracts.UpsertGroup) (int, error)
	// FindByID returns a group by id (tenant-scoped).
	FindByID(ctx context.Context, id string) (*entity.Group, error)
	// FindByJID returns a group by its JID (tenant-scoped). Used by Domain 2's
	// attend gate before creating a conversation for a group message.
	FindByJID(ctx context.Context, groupJID string) (*entity.Group, error)
	// List returns a keyset page of groups matching the filter (text search on
	// name+description when Q is set).
	List(ctx context.Context, f contracts.ListFilter, page shared.PageRequest) ([]*entity.Group, error)
	// SetAttend toggles the attendance filter on a group.
	SetAttend(ctx context.Context, id string, attend bool) (*entity.Group, error)
}

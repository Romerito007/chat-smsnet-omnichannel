// Package repository declares the conversationtools persistence contracts. All
// reads are tenant-scoped from the context.
package repository

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversationtools/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// TagRepository persists tags.
type TagRepository interface {
	Create(ctx context.Context, t *entity.Tag) error
	Update(ctx context.Context, t *entity.Tag) error
	Delete(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (*entity.Tag, error)
	List(ctx context.Context, page shared.PageRequest) ([]*entity.Tag, error)
	// FindByIDs returns the tags matching the ids (tenant-scoped). Used to
	// validate tag ids before applying them to a conversation.
	FindByIDs(ctx context.Context, ids []string) ([]*entity.Tag, error)
}

// CannedResponseRepository persists canned responses.
type CannedResponseRepository interface {
	Create(ctx context.Context, c *entity.CannedResponse) error
	Update(ctx context.Context, c *entity.CannedResponse) error
	Delete(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (*entity.CannedResponse, error)
	FindByShortcut(ctx context.Context, shortcut string) (*entity.CannedResponse, error)
	List(ctx context.Context, page shared.PageRequest) ([]*entity.CannedResponse, error)
}

// CloseReasonRepository persists close reasons.
type CloseReasonRepository interface {
	Create(ctx context.Context, c *entity.CloseReason) error
	Update(ctx context.Context, c *entity.CloseReason) error
	Delete(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (*entity.CloseReason, error)
	// FindByIDs returns the close reasons matching the ids (tenant-scoped). Used to
	// resolve reason names for the conversations report; missing ids are absent.
	FindByIDs(ctx context.Context, ids []string) ([]*entity.CloseReason, error)
	List(ctx context.Context, page shared.PageRequest) ([]*entity.CloseReason, error)
}

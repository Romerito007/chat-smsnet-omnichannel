// Package repository declares the conversations persistence contracts.
// Implementations live in infra/database/mongodb/repositories/conversations.
// Every method is tenant-scoped via the context.
package repository

import (
	"context"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// ConversationRepository persists conversations.
type ConversationRepository interface {
	Create(ctx context.Context, c *entity.Conversation) error
	Update(ctx context.Context, c *entity.Conversation) error
	FindByID(ctx context.Context, id string) (*entity.Conversation, error)
	// FindOpenByContactChannel returns the most recent non-closed conversation
	// for a contact on a channel, or a not_found AppError. Used by inbound to
	// reuse an open conversation instead of creating a new one.
	FindOpenByContactChannel(ctx context.Context, contactID, channel string) (*entity.Conversation, error)
	// List returns conversations matching the filter and visibility, ordered by
	// updated_at desc (keyset). Over-fetches by one for has_more detection.
	List(ctx context.Context, filter contracts.ListFilter, vis contracts.Visibility, page shared.PageRequest) ([]*entity.Conversation, error)
	// ListInactiveOpen returns up to limit non-closed conversations whose last
	// activity is at or before idleBefore (tenant-scoped). Used by the
	// close-inactive job.
	ListInactiveOpen(ctx context.Context, idleBefore time.Time, limit int) ([]*entity.Conversation, error)
}

// MessageRepository persists messages.
type MessageRepository interface {
	Create(ctx context.Context, m *entity.Message) error
	Update(ctx context.Context, m *entity.Message) error
	FindByID(ctx context.Context, id string) (*entity.Message, error)
	// ListByConversation returns non-deleted messages, newest first (keyset).
	ListByConversation(ctx context.Context, conversationID string, page shared.PageRequest) ([]*entity.Message, error)
	// LatestByConversation returns the most recent non-deleted message of a
	// conversation (for list previews), or NotFound when the thread is empty.
	LatestByConversation(ctx context.Context, conversationID string) (*entity.Message, error)
}

// EventRepository persists conversation timeline events.
type EventRepository interface {
	Create(ctx context.Context, e *entity.ConversationEvent) error
	ListByConversation(ctx context.Context, conversationID string, page shared.PageRequest) ([]*entity.ConversationEvent, error)
}

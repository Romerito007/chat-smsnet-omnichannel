// Package repository declares the conversations persistence contracts.
// Implementations live in infra/database/mongodb/repositories/conversations.
// Every method is tenant-scoped via the context.
package repository

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// ConversationRepository persists conversations.
type ConversationRepository interface {
	Create(ctx context.Context, c *entity.Conversation) error
	Update(ctx context.Context, c *entity.Conversation) error
	FindByID(ctx context.Context, id string) (*entity.Conversation, error)
	// List returns conversations matching the filter and visibility, ordered by
	// updated_at desc (keyset). Over-fetches by one for has_more detection.
	List(ctx context.Context, filter contracts.ListFilter, vis contracts.Visibility, page shared.PageRequest) ([]*entity.Conversation, error)
}

// MessageRepository persists messages.
type MessageRepository interface {
	Create(ctx context.Context, m *entity.Message) error
	Update(ctx context.Context, m *entity.Message) error
	FindByID(ctx context.Context, id string) (*entity.Message, error)
	// ListByConversation returns non-deleted messages, newest first (keyset).
	ListByConversation(ctx context.Context, conversationID string, page shared.PageRequest) ([]*entity.Message, error)
}

// EventRepository persists conversation timeline events.
type EventRepository interface {
	Create(ctx context.Context, e *entity.ConversationEvent) error
	ListByConversation(ctx context.Context, conversationID string, page shared.PageRequest) ([]*entity.ConversationEvent, error)
}

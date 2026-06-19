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
	// FindByIDs batch-loads conversations by id within the tenant (missing ids
	// absent), used to hydrate the SLA breach check without a FindByID per tracking.
	FindByIDs(ctx context.Context, ids []string) ([]*entity.Conversation, error)
	// FindOpenByContactChannelID returns the most recent non-closed conversation
	// for a contact on a specific channel CONNECTION (by id), or a not_found
	// AppError. Keyed by the connection id so two connections of the same type stay
	// distinct conversations.
	FindOpenByContactChannelID(ctx context.Context, contactID, channelID string) (*entity.Conversation, error)
	// FindLastByContactChannelID returns the most recent conversation for a contact
	// on a specific channel CONNECTION (by id) REGARDLESS of status (open or
	// closed), or a not_found AppError. Used by single-mode inbound to reopen the
	// last conversation instead of creating a new one.
	FindLastByContactChannelID(ctx context.Context, contactID, channelID string) (*entity.Conversation, error)
	// FindOpenByContact returns the most recent non-closed conversation for a contact
	// across ANY channel connection, or a not_found AppError. Used for GROUP messages:
	// a group is reachable via every connected number that is a member, so the group
	// thread is keyed by the (group) contact, not by the receiving connection.
	FindOpenByContact(ctx context.Context, contactID string) (*entity.Conversation, error)
	// FindLastByContact returns the most recent conversation for a contact across ANY
	// channel connection REGARDLESS of status, or a not_found AppError. Used to reopen
	// a group's last (closed) thread instead of creating a new one.
	FindLastByContact(ctx context.Context, contactID string) (*entity.Conversation, error)
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
	// conversation, or NotFound when the thread is empty. Used to recompute the
	// denormalized last-message snapshot when the latest message is deleted.
	LatestByConversation(ctx context.Context, conversationID string) (*entity.Message, error)
	// FindByExternalMessageID returns a conversation's message by its external
	// (channel) id, or NotFound. Used to resolve an interactive reply's context.id
	// back to the internal id of the menu message we sent.
	FindByExternalMessageID(ctx context.Context, conversationID, externalID string) (*entity.Message, error)
}

// EventRepository persists conversation timeline events.
type EventRepository interface {
	Create(ctx context.Context, e *entity.ConversationEvent) error
	ListByConversation(ctx context.Context, conversationID string, page shared.PageRequest) ([]*entity.ConversationEvent, error)
}

// ProtocolCounterRepository hands out the next protocol sequence per (tenant,
// year) atomically (no count-and-add race).
type ProtocolCounterRepository interface {
	// NextSequence atomically increments and returns the next sequence for the
	// tenant + year (starting at 1 for a year not yet seen).
	NextSequence(ctx context.Context, tenantID string, year int) (int64, error)
}

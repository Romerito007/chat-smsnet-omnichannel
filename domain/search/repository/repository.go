// Package repository declares the search index port. One backend (Mongo today)
// implements it; swapping to a dedicated engine means a new implementation.
package repository

import (
	"context"

	contactentity "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/entity"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/search/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// Index is the search backend. All reads are tenant-scoped from the context;
// conversation visibility is applied at the index level for conversations and by
// the service (via the loaders below) for contacts/messages.
type Index interface {
	// SearchConversations applies the filters + visibility + keyset directly,
	// over-fetching by one to detect more pages.
	SearchConversations(ctx context.Context, f contracts.ConversationFilter, vis contracts.Visibility, page shared.PageRequest) ([]*conventity.Conversation, error)

	// SearchContactsText returns text matches (name/document/phones) after the
	// cursor, up to scanLimit (the service post-filters by visibility).
	SearchContactsText(ctx context.Context, query string, cur shared.Cursor, scanLimit int) ([]*contactentity.Contact, error)

	// SearchMessagesText returns text matches on messages.text after the cursor,
	// up to scanLimit, excluding deleted messages (the service post-filters by
	// conversation visibility).
	SearchMessagesText(ctx context.Context, query, conversationID string, cur shared.Cursor, scanLimit int) ([]*conventity.Message, error)

	// FindConversation loads a conversation (tenant-scoped) for visibility checks.
	FindConversation(ctx context.Context, id string) (*conventity.Conversation, error)

	// HasVisibleConversationForContact reports whether the actor can see any
	// conversation with the contact (used to scope contact results).
	HasVisibleConversationForContact(ctx context.Context, contactID string, vis contracts.Visibility) (bool, error)
}

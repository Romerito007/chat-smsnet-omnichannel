// Package contracts holds the search domain's query types, the visibility model
// and the SearchService abstraction (so the Mongo-backed MVP can be swapped for a
// dedicated engine later).
package contracts

import (
	"context"
	"time"

	contactentity "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/entity"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// Visibility is the actor's conversation-visibility scope. All-scope sees the
// whole tenant; otherwise the actor sees conversations assigned to them or in
// their sectors (a supervisor sees their sector).
type Visibility struct {
	All       bool
	SectorIDs []string
	UserID    string
}

// CanSee reports whether the actor may see a conversation.
func (v Visibility) CanSee(conv *conventity.Conversation) bool {
	if v.All {
		return true
	}
	if conv == nil {
		return false
	}
	if conv.AssignedTo != "" && conv.AssignedTo == v.UserID {
		return true
	}
	for _, s := range v.SectorIDs {
		if s != "" && s == conv.SectorID {
			return true
		}
	}
	return false
}

// ConversationFilter combines the conversation search filters. Empty fields are
// ignored; From/To bound the creation time.
type ConversationFilter struct {
	Status     string
	SectorID   string
	AssignedTo string
	Channel    string
	Tag        string
	Priority   string
	SLAStatus  string // breached | at_risk | met | running
	From       *time.Time
	To         *time.Time
}

// MessageFilter is the message search query.
type MessageFilter struct {
	Query          string
	ConversationID string
}

// Result is a page of search hits with a cursor to continue.
type Result[T any] struct {
	Items      []T
	NextCursor string
}

// SearchService is the search abstraction. The Mongo implementation backs the
// MVP; a dedicated engine can implement the same interface later. Every method
// enforces tenant + the actor's visibility scope.
type SearchService interface {
	SearchConversations(ctx context.Context, f ConversationFilter, page shared.PageRequest) (Result[*conventity.Conversation], error)
	SearchContacts(ctx context.Context, query string, page shared.PageRequest) (Result[*contactentity.Contact], error)
	SearchMessages(ctx context.Context, f MessageFilter, page shared.PageRequest) (Result[*conventity.Message], error)
}

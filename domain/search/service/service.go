// Package service implements the SearchService over a pluggable Index. It builds
// the actor's visibility from the auth context and enforces it: conversations
// are scoped at the index, contacts/messages are post-filtered so a user never
// sees what they could not see in the inbox (e.g. messages, including internal
// notes, of conversations outside their scope).
package service

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	contactentity "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/entity"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/search/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/search/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// maxScan bounds the over-fetch used when post-filtering by visibility.
const maxScan = 200

// Service is the Mongo-backed SearchService.
type Service struct {
	index repository.Index
}

// NewService builds the service.
func NewService(index repository.Index) *Service {
	return &Service{index: index}
}

// SearchConversations applies filters + visibility at the index and paginates by
// keyset on updated_at.
func (s *Service) SearchConversations(ctx context.Context, f contracts.ConversationFilter, page shared.PageRequest) (contracts.Result[*conventity.Conversation], error) {
	vis, err := s.visibility(ctx)
	if err != nil {
		return contracts.Result[*conventity.Conversation]{}, err
	}
	page = page.Normalize()
	items, err := s.index.SearchConversations(ctx, f, vis, page)
	if err != nil {
		return contracts.Result[*conventity.Conversation]{}, err
	}
	next := ""
	if len(items) > page.Limit {
		items = items[:page.Limit]
		last := items[len(items)-1]
		next = shared.Cursor{CreatedAt: last.UpdatedAt.UnixMilli(), ID: last.ID}.Encode()
	}
	return contracts.Result[*conventity.Conversation]{Items: items, NextCursor: next}, nil
}

// SearchContacts text-searches contacts and scopes them: an actor without
// all-scope only sees a contact they share a visible conversation with.
func (s *Service) SearchContacts(ctx context.Context, query string, page shared.PageRequest) (contracts.Result[*contactentity.Contact], error) {
	vis, err := s.visibility(ctx)
	if err != nil {
		return contracts.Result[*contactentity.Contact]{}, err
	}
	page = page.Normalize()
	cur, err := shared.DecodeCursor(page.Cursor)
	if err != nil {
		return contracts.Result[*contactentity.Contact]{}, err
	}

	visible := func(c *contactentity.Contact) bool {
		if vis.All {
			return true
		}
		ok, _ := s.index.HasVisibleConversationForContact(ctx, c.ID, vis)
		return ok
	}
	cursorOf := func(c *contactentity.Contact) shared.Cursor {
		return shared.Cursor{CreatedAt: c.CreatedAt.UnixMilli(), ID: c.ID}
	}
	items, next, err := filteredGeneric(page, vis.All,
		func(scan int) ([]*contactentity.Contact, error) {
			return s.index.SearchContactsText(ctx, query, cur, scan)
		}, visible, cursorOf)
	if err != nil {
		return contracts.Result[*contactentity.Contact]{}, err
	}
	return contracts.Result[*contactentity.Contact]{Items: items, NextCursor: next}, nil
}

// SearchMessages text-searches messages and returns only those whose conversation
// the actor can see.
func (s *Service) SearchMessages(ctx context.Context, f contracts.MessageFilter, page shared.PageRequest) (contracts.Result[*conventity.Message], error) {
	vis, err := s.visibility(ctx)
	if err != nil {
		return contracts.Result[*conventity.Message]{}, err
	}
	page = page.Normalize()
	cur, err := shared.DecodeCursor(page.Cursor)
	if err != nil {
		return contracts.Result[*conventity.Message]{}, err
	}

	convCache := map[string]*conventity.Conversation{}
	visible := func(m *conventity.Message) bool {
		if vis.All {
			return true
		}
		conv, ok := convCache[m.ConversationID]
		if !ok {
			conv, _ = s.index.FindConversation(ctx, m.ConversationID)
			convCache[m.ConversationID] = conv
		}
		return vis.CanSee(conv)
	}
	cursorOf := func(m *conventity.Message) shared.Cursor {
		return shared.Cursor{CreatedAt: m.CreatedAt.UnixMilli(), ID: m.ID}
	}
	items, next, err := filteredGeneric(page, vis.All,
		func(scan int) ([]*conventity.Message, error) {
			return s.index.SearchMessagesText(ctx, f.Query, f.ConversationID, cur, scan)
		}, visible, cursorOf)
	if err != nil {
		return contracts.Result[*conventity.Message]{}, err
	}
	return contracts.Result[*conventity.Message]{Items: items, NextCursor: next}, nil
}

// filtered runs a keyset scan and post-filters by a visibility predicate,
// producing a page with a cursor that always makes progress. When allScope is
// true no post-filtering happens (a tight limit+1 fetch is used).
func filteredGeneric[T any](
	page shared.PageRequest,
	allScope bool,
	fetch func(scan int) ([]T, error),
	visible func(T) bool,
	cursorOf func(T) shared.Cursor,
) ([]T, string, error) {
	limit := page.Limit
	scan := limit + 1
	if !allScope {
		scan = limit * 5
		if scan > maxScan {
			scan = maxScan
		}
		if scan < limit+1 {
			scan = limit + 1
		}
	}
	raw, err := fetch(scan)
	if err != nil {
		return nil, "", err
	}
	scannedAll := len(raw) < scan

	kept := make([]T, 0, limit)
	for i := range raw {
		item := raw[i]
		if !visible(item) {
			continue
		}
		kept = append(kept, item)
		if len(kept) == limit {
			// More may exist after this item.
			if i < len(raw)-1 || !scannedAll {
				return kept, cursorOf(item).Encode(), nil
			}
			return kept, "", nil
		}
	}
	// Consumed the whole batch; a full batch means more candidates remain.
	if !scannedAll && len(raw) > 0 {
		return kept, cursorOf(raw[len(raw)-1]).Encode(), nil
	}
	return kept, "", nil
}

func (s *Service) visibility(ctx context.Context) (contracts.Visibility, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return contracts.Visibility{}, err
	}
	ac, ok := authz.FromContext(ctx)
	if !ok {
		return contracts.Visibility{}, apperror.Unauthorized("authentication required")
	}
	return contracts.Visibility{
		All:       ac.SectorScope == authz.ScopeAll,
		SectorIDs: ac.SectorIDs,
		UserID:    ac.UserID,
	}, nil
}

var _ contracts.SearchService = (*Service)(nil)

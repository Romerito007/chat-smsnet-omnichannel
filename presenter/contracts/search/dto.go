// Package search holds the response DTOs for the search endpoints.
package search

import (
	"time"

	contactentity "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/entity"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
)

// Page is the search response envelope.
type Page[T any] struct {
	Data []T      `json:"data"`
	Page PageInfo `json:"page"`
}

// PageInfo carries the cursor to continue.
type PageInfo struct {
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}

// NewPage builds a search page envelope.
func NewPage[T any](data []T, nextCursor string) Page[T] {
	if data == nil {
		data = []T{}
	}
	return Page[T]{Data: data, Page: PageInfo{NextCursor: nextCursor, HasMore: nextCursor != ""}}
}

// ConversationHit is the conversation search result.
type ConversationHit struct {
	ID            string    `json:"id"`
	ContactID     string    `json:"contact_id"`
	Channel       string    `json:"channel"`
	SectorID      string    `json:"sector_id,omitempty"`
	Status        string    `json:"status"`
	AssignedTo    string    `json:"assigned_to,omitempty"`
	Priority      string    `json:"priority"`
	Tags          []string  `json:"tags,omitempty"`
	LastMessageAt time.Time `json:"last_message_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func NewConversationHit(c *conventity.Conversation) ConversationHit {
	return ConversationHit{
		ID: c.ID, ContactID: c.ContactID, Channel: c.Channel, SectorID: c.SectorID,
		Status: string(c.Status), AssignedTo: c.AssignedTo, Priority: string(c.Priority),
		Tags: c.Tags, LastMessageAt: c.LastMessageAt, UpdatedAt: c.UpdatedAt,
	}
}

func NewConversationHits(items []*conventity.Conversation) []ConversationHit {
	out := make([]ConversationHit, 0, len(items))
	for _, c := range items {
		out = append(out, NewConversationHit(c))
	}
	return out
}

// ContactHit is the contact search result.
type ContactHit struct {
	ID       string `json:"id"`
	Name     string `json:"name,omitempty"`
	Phone    string `json:"phone,omitempty"`
	Document string `json:"document,omitempty"`
}

func NewContactHit(c *contactentity.Contact) ContactHit {
	return ContactHit{ID: c.ID, Name: c.Name, Phone: c.Phone, Document: c.Document}
}

func NewContactHits(items []*contactentity.Contact) []ContactHit {
	out := make([]ContactHit, 0, len(items))
	for _, c := range items {
		out = append(out, NewContactHit(c))
	}
	return out
}

// MessageHit is the message search result.
type MessageHit struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	SenderType     string    `json:"sender_type"`
	Direction      string    `json:"direction"`
	Text           string    `json:"text"`
	CreatedAt      time.Time `json:"created_at"`
}

func NewMessageHit(m *conventity.Message) MessageHit {
	return MessageHit{
		ID: m.ID, ConversationID: m.ConversationID, SenderType: string(m.SenderType),
		Direction: string(m.Direction), Text: m.Text, CreatedAt: m.CreatedAt,
	}
}

func NewMessageHits(items []*conventity.Message) []MessageHit {
	out := make([]MessageHit, 0, len(items))
	for _, m := range items {
		out = append(out, NewMessageHit(m))
	}
	return out
}

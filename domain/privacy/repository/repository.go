// Package repository declares the privacy persistence port (Store) plus the
// plain data structs used to assemble an export bundle. The structs are
// domain-local (decoupled from the conversations/csat/contacts entities) so the
// privacy domain takes no dependency on those packages; the Mongo implementation
// maps documents straight into them.
package repository

import (
	"context"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/privacy/entity"
)

// Store is the privacy domain's persistence port. The Mongo implementation reads
// across the contacts/conversations/messages/csat collections (all tenant-scoped
// via context) to assemble exports, anonymize PII and apply retention.
type Store interface {
	// --- Retention policy ---
	GetRetention(ctx context.Context) (*entity.RetentionPolicy, error)
	SaveRetention(ctx context.Context, p *entity.RetentionPolicy) error

	// --- Export requests ---
	CreateExport(ctx context.Context, e *entity.ExportRequest) error
	UpdateExport(ctx context.Context, e *entity.ExportRequest) error
	FindExport(ctx context.Context, id string) (*entity.ExportRequest, error)

	// CollectBundle assembles the contact's chat data (contact + conversations +
	// messages + CSAT). It returns a not_found error when the contact does not
	// exist in the tenant. It never includes external provider data.
	CollectBundle(ctx context.Context, contactID string) (*ExportBundle, error)

	// --- Anonymization ---
	// AnonymizeContact overwrites the contact's PII fields and clears channel
	// identity handles, keeping the row and its id (integrity). Not found when the
	// contact is missing.
	AnonymizeContact(ctx context.Context, contactID string, a Anonymized) error
	// UpdateMessageText overwrites one message's text (used to mask PII in place,
	// iterating the bundle's messages).
	UpdateMessageText(ctx context.Context, id, text string) error

	// --- Legal hold ---
	// HasActiveLegalHold reports whether the contact has a legal hold in force at
	// `at` ("não anonimizar dados sob obrigação legal antes do prazo").
	HasActiveLegalHold(ctx context.Context, contactID string, at time.Time) (bool, error)

	// --- Retention application ---
	// ApplyRetention deletes data older than each policy's cutoff (computed from
	// `now`), skipping anything tied to a contact under an active legal hold.
	ApplyRetention(ctx context.Context, p entity.RetentionPolicy, now time.Time) (RetentionResult, error)
}

// Anonymized carries the replacement PII values written to a contact.
type Anonymized struct {
	Name     string
	Phone    string
	Document string
}

// ExportBundle is the serialized contact data set.
type ExportBundle struct {
	GeneratedAt   time.Time          `json:"generated_at"`
	Contact       ContactData        `json:"contact"`
	Conversations []ConversationData `json:"conversations"`
	CSAT          []CSATData         `json:"csat"`
}

// ContactData is the contact section of the bundle.
type ContactData struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Phone      string         `json:"phone"`
	Document   string         `json:"document"`
	Identities []IdentityData `json:"identities"`
	// Anonymized reports whether the contact's PII was already scrubbed, so the
	// anonymize use case can short-circuit idempotently.
	Anonymized bool      `json:"anonymized,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// IdentityData is one channel identity.
type IdentityData struct {
	Channel    string `json:"channel"`
	ExternalID string `json:"external_id"`
}

// ConversationData is one conversation with its messages.
type ConversationData struct {
	ID        string        `json:"id"`
	Channel   string        `json:"channel"`
	Status    string        `json:"status"`
	CreatedAt time.Time     `json:"created_at"`
	ClosedAt  *time.Time    `json:"closed_at,omitempty"`
	Messages  []MessageData `json:"messages"`
}

// MessageData is one message in the bundle.
type MessageData struct {
	ID         string    `json:"id"`
	Direction  string    `json:"direction"`
	SenderType string    `json:"sender_type"`
	Type       string    `json:"type"`
	Text       string    `json:"text"`
	CreatedAt  time.Time `json:"created_at"`
}

// CSATData is one CSAT response in the bundle.
type CSATData struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	Score          *int      `json:"score,omitempty"`
	Comment        string    `json:"comment,omitempty"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

// RetentionResult reports how many documents each category deleted.
type RetentionResult struct {
	Messages            int
	ClosedConversations int
	TechnicalLogs       int
	AuditLogs           int
	Notifications       int
}

// Total is the sum across categories.
func (r RetentionResult) Total() int {
	return r.Messages + r.ClosedConversations + r.TechnicalLogs + r.AuditLogs + r.Notifications
}

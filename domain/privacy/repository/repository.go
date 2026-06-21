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
// via context) to assemble exports, erase contacts and apply retention.
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

	// --- Erasure (right to be forgotten) ---
	// LinkedDeals lists the CRM deals tied to the contact (directly by contact_id
	// or through any of the contact's conversations). The erasure use case surfaces
	// these so the company can review the link before the contact is destroyed
	// ("sistema avisa → empresa trata o vínculo → exclusão prossegue").
	LinkedDeals(ctx context.Context, contactID string) ([]DealLink, error)
	// EraseContact hard-deletes the contact and every satellite document carrying
	// its personal data / communications (conversations, messages, events,
	// attachments, CSAT, SLA, MCP, copilot, rule-eval and provider-query logs,
	// inbound messages, export requests), returning the blob/export storage keys it
	// detached so the caller can purge the physical files. When unlinkDeals is true
	// it first severs the contact from its deals (clearing contact_id and pulling
	// the conversation ids) WITHOUT deleting the deals — the CRM record is kept for
	// the company. It returns a not_found error when the contact is missing.
	EraseContact(ctx context.Context, contactID string, unlinkDeals bool) (EraseResult, error)

	// --- Legal hold ---
	// HasActiveLegalHold reports whether the contact has a legal hold in force at
	// `at` ("não apagar dados sob obrigação legal antes do prazo").
	HasActiveLegalHold(ctx context.Context, contactID string, at time.Time) (bool, error)

	// --- Retention application ---
	// ApplyRetention deletes data older than each policy's cutoff (computed from
	// `now`), skipping anything tied to a contact under an active legal hold.
	ApplyRetention(ctx context.Context, p entity.RetentionPolicy, now time.Time) (RetentionResult, error)
}

// DealLink identifies a CRM deal tied to a contact being erased, surfaced in the
// 409 warning so the company can review the link first. Only the id and title
// are exposed — never the deal's own data.
type DealLink struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// EraseResult reports what a contact erasure removed (for the audit trail) and
// the physical storage keys the caller must still purge.
type EraseResult struct {
	Conversations     int
	Messages          int
	Events            int
	Attachments       int
	CSAT              int
	SLATrackings      int
	MCPApprovals      int
	MCPCallLogs       int
	CopilotLogs       int
	RuleEvalLogs      int
	InboundMessages   int
	ProviderQueryLogs int
	Exports           int
	DealsUnlinked     int
	// BlobKeys are attachment media object keys (incl. the contact avatar) to
	// delete from the attachments BlobStore.
	BlobKeys []string
	// ExportKeys are export-bundle object keys to delete from the FileStore.
	ExportKeys []string
}

// Documents is the count of database rows removed (excludes blobs/exports files
// and deal unlinks, which are not deletions).
func (r EraseResult) Documents() int {
	return r.Conversations + r.Messages + r.Events + r.Attachments + r.CSAT +
		r.SLATrackings + r.MCPApprovals + r.MCPCallLogs + r.CopilotLogs +
		r.RuleEvalLogs + r.InboundMessages + r.ProviderQueryLogs + r.Exports
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
	// SatelliteDocs counts the conversation-scoped documents (messages, events,
	// attachments, CSAT, SLA, MCP, copilot, rule-eval, provider and inbound logs)
	// cascade-deleted alongside closed conversations, so retention never strands
	// them.
	SatelliteDocs int
	// BlobKeys are attachment media object keys to purge from the BlobStore for
	// the conversations retention deleted.
	BlobKeys []string
}

// Total is the sum across categories.
func (r RetentionResult) Total() int {
	return r.Messages + r.ClosedConversations + r.TechnicalLogs + r.AuditLogs +
		r.Notifications + r.SatelliteDocs
}

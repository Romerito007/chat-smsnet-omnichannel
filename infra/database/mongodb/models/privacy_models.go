package models

import "time"

// RetentionPolicy is the BSON document for a tenant's data-retention policy
// (one per tenant; _id is the tenant id).
type RetentionPolicy struct {
	ID                      string    `bson:"_id"`
	TenantID                string    `bson:"tenant_id"`
	MessagesDays            int       `bson:"messages_days"`
	ClosedConversationsDays int       `bson:"closed_conversations_days"`
	TechnicalLogsDays       int       `bson:"technical_logs_days"`
	AuditLogsDays           int       `bson:"audit_logs_days"`
	NotificationsDays       int       `bson:"notifications_days"`
	UpdatedAt               time.Time `bson:"updated_at"`
}

// PrivacyExport is the BSON document for a contact data-export request.
type PrivacyExport struct {
	ID          string     `bson:"_id"`
	TenantID    string     `bson:"tenant_id"`
	ContactID   string     `bson:"contact_id"`
	Status      string     `bson:"status"`
	RequestedBy string     `bson:"requested_by,omitempty"`
	StorageKey  string     `bson:"storage_key,omitempty"`
	DownloadURL string     `bson:"download_url,omitempty"`
	ExpiresAt   time.Time  `bson:"expires_at,omitempty"`
	Error       string     `bson:"error,omitempty"`
	CreatedAt   time.Time  `bson:"created_at"`
	CompletedAt *time.Time `bson:"completed_at,omitempty"`
}

// The following are read-only projections used to assemble an export bundle from
// the contacts/conversations/messages/csat collections.

// PrivacyContact projects the contact fields needed for the export bundle.
type PrivacyContact struct {
	ID         string            `bson:"_id"`
	Name       string            `bson:"name"`
	Phone      string            `bson:"phone"`
	Document   string            `bson:"document"`
	Identities []ChannelIdentity `bson:"identities"`
	Anonymized bool              `bson:"anonymized"`
	CreatedAt  time.Time         `bson:"created_at"`
}

// PrivacyConversation projects the conversation fields needed for the bundle.
type PrivacyConversation struct {
	ID        string     `bson:"_id"`
	Channel   string     `bson:"channel"`
	Status    string     `bson:"status"`
	CreatedAt time.Time  `bson:"created_at"`
	ClosedAt  *time.Time `bson:"closed_at,omitempty"`
}

// PrivacyMessage projects the message fields needed for the bundle.
type PrivacyMessage struct {
	ID          string     `bson:"_id"`
	Direction   string     `bson:"direction"`
	SenderType  string     `bson:"sender_type"`
	MessageType string     `bson:"message_type"`
	Text        string     `bson:"text"`
	CreatedAt   time.Time  `bson:"created_at"`
	DeletedAt   *time.Time `bson:"deleted_at,omitempty"`
}

// PrivacyCSAT projects the CSAT response fields needed for the bundle.
type PrivacyCSAT struct {
	ID             string    `bson:"_id"`
	ConversationID string    `bson:"conversation_id"`
	Score          *int      `bson:"score,omitempty"`
	Comment        string    `bson:"comment,omitempty"`
	Status         string    `bson:"status"`
	CreatedAt      time.Time `bson:"created_at"`
}

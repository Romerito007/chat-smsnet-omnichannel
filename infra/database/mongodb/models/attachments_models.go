package models

import "time"

// AttachmentRecord is the BSON document for a conversation attachment. It is
// named AttachmentRecord to avoid colliding with Attachment, the message
// attachment sub-document in conversations_models.go.
type AttachmentRecord struct {
	ID              string    `bson:"_id"`
	TenantID        string    `bson:"tenant_id"`
	ConversationID  string    `bson:"conversation_id"`
	MessageID       string    `bson:"message_id,omitempty"`
	Filename        string    `bson:"filename"`
	ContentType     string    `bson:"content_type"`
	Size            int64     `bson:"size"`
	StorageProvider string    `bson:"storage_provider"`
	StorageKey      string    `bson:"storage_key"`
	SignedURL       string    `bson:"signed_url,omitempty"`
	Status          string    `bson:"status"`
	CreatedBy       string    `bson:"created_by,omitempty"`
	CreatedAt       time.Time `bson:"created_at"`
}

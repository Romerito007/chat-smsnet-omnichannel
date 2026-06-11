// Package entity holds the Attachment aggregate: a media file linked to a
// conversation (and optionally a specific message), stored behind a signed URL.
package entity

import "time"

// Status is the lifecycle of an attachment.
type Status string

const (
	// StatusPending is the initial state: an upload URL was issued but the client
	// has not confirmed the upload yet.
	StatusPending Status = "pending"
	// StatusReady means the upload was confirmed and the file is downloadable.
	StatusReady Status = "ready"
)

// Attachment is a media file belonging to a conversation. The bytes live in the
// configured storage backend (local or S3-compatible) under StorageKey; access
// is always mediated by a short-lived signed URL after a conversation-access
// check — the raw object is never served directly.
type Attachment struct {
	ID              string
	TenantID        string
	ConversationID  string
	MessageID       string
	Filename        string
	ContentType     string
	Size            int64
	StorageProvider string
	StorageKey      string
	SignedURL       string
	Status          Status
	CreatedBy       string
	CreatedAt       time.Time
}

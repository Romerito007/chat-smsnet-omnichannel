package entity

import "time"

// ExportStatus is the lifecycle of a data-export request.
type ExportStatus string

const (
	// ExportPending is the initial state: the request is enqueued for the worker.
	ExportPending ExportStatus = "pending"
	// ExportReady means the file was assembled and a temporary signed URL is set.
	ExportReady ExportStatus = "ready"
	// ExportFailed means assembly failed; Error explains why.
	ExportFailed ExportStatus = "failed"
)

// ExportRequest tracks a contact data-export ("right of access"). The assembled
// file holds only the contact's chat data (contact, conversations, messages,
// CSAT) — never external provider data, which is not persisted. The download is
// exposed through a temporary signed URL.
type ExportRequest struct {
	ID          string
	TenantID    string
	ContactID   string
	Status      ExportStatus
	RequestedBy string
	// StorageKey is the opaque object key under which the file is stored.
	StorageKey string
	// DownloadURL is the temporary signed URL (set when Status==ready).
	DownloadURL string
	// ExpiresAt is when the signed URL stops working.
	ExpiresAt   time.Time
	Error       string
	CreatedAt   time.Time
	CompletedAt *time.Time
}

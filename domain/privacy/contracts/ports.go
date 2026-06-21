package contracts

import "time"

// FileStore persists export artifacts and mints temporary, signed download URLs.
// Implemented by infra/storage (local filesystem + HMAC-signed token in the MVP;
// an object-store backend can replace it without touching the domain).
type FileStore interface {
	// Save writes data under key, returning nil on success.
	Save(key string, data []byte) error
	// SignedURL builds a temporary download URL for key valid for ttl, returning
	// the URL and its absolute expiry.
	SignedURL(key string, ttl time.Duration) (url string, expiresAt time.Time, err error)
	// Resolve validates a signed token (from the URL) and returns the object key.
	// It fails for tampered or expired tokens.
	Resolve(token string) (key string, err error)
	// Open returns the stored bytes and a suggested download filename for key.
	Open(key string) (data []byte, filename string, err error)
	// Delete removes the object under key. A missing object is not an error
	// (best-effort cleanup), so a contact erasure can purge export bundles
	// idempotently.
	Delete(key string) error
}

// BlobStore deletes attachment media blobs by their storage key. It is the
// narrow slice of the attachments storage backend the privacy domain needs to
// purge media on contact erasure (LGPD). Implemented by the configured
// attachments storage (local filesystem or S3).
type BlobStore interface {
	// Delete removes the blob under key; a missing blob is not an error.
	Delete(key string) error
}

// ExportEnqueuer schedules the privacy.export job. Implemented by the infra
// Asynq enqueuer.
type ExportEnqueuer interface {
	EnqueueExport(task ExportTask) error
}

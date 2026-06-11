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
}

// ExportEnqueuer schedules the privacy.export job. Implemented by the infra
// Asynq enqueuer.
type ExportEnqueuer interface {
	EnqueueExport(task ExportTask) error
}

// Package contracts holds the attachments domain ports (storage backend) and
// command inputs.
package contracts

import "time"

// CreateUploadURL is the POST /v1/attachments/upload-url input. Exactly one
// target applies: a conversation attachment (ConversationID set) or an avatar
// (Avatar set, no conversation).
type CreateUploadURL struct {
	ConversationID string
	Filename       string
	ContentType    string
	Size           int64
	// Avatar, when set, issues a conversation-less avatar upload: the key is
	// namespaced avatars/{tenant}/{OwnerType}/{OwnerID}/{filename}, the content
	// type is restricted to image/*, and the avatar size limit applies.
	Avatar *AvatarTarget
}

// AvatarTarget identifies the owner of an avatar upload. OwnerType is the owner
// collection ("contacts" or "users"); OwnerID is that owner's id (tenant-scoped).
type AvatarTarget struct {
	OwnerType string
	OwnerID   string
}

// ConfirmUpload is the POST /v1/attachments/confirm input. MessageID is optional:
// when set, the attachment is linked to that message.
type ConfirmUpload struct {
	AttachmentID string
	MessageID    string
}

// UploadTarget tells the client where and how to upload the bytes directly to the
// storage backend.
type UploadTarget struct {
	URL       string
	Method    string
	Headers   map[string]string
	ExpiresAt time.Time
}

// DownloadResult is the outcome of resolving a download. Exactly one of
// RedirectURL (S3-compatible presigned URL) or Data (local bytes) is set.
type DownloadResult struct {
	RedirectURL string
	Data        []byte
	ContentType string
	Filename    string
}

// Storage abstracts the attachment blob backend (local filesystem or
// S3-compatible). Implemented in infra/storage. The domain never touches the
// filesystem or an S3 client directly.
type Storage interface {
	// Provider identifies the backend ("local" | "s3").
	Provider() string
	// SignUpload returns a temporary target the client uploads the bytes to.
	SignUpload(key, contentType string, size int64, ttl time.Duration) (UploadTarget, error)
	// Download resolves the object for serving: bytes for local, a short-lived
	// presigned redirect URL for S3.
	Download(key, filename string, ttl time.Duration) (DownloadResult, error)
	// Put stores bytes under key. Used by the local backend's blob upload endpoint;
	// the S3 backend uploads directly from the client and may return an error.
	Put(key, contentType string, data []byte) error
	// Exists reports whether the object was actually uploaded — used by confirm to
	// reject a confirmation when the client never PUT the bytes.
	Exists(key string) (bool, error)
}

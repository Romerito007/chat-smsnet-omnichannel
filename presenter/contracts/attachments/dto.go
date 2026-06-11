// Package attachments holds the request/response DTOs for the attachments
// endpoints.
package attachments

import (
	"time"

	acontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/attachments/contracts"
	aentity "github.com/romerito007/chat-smsnet-omnichannel/domain/attachments/entity"
)

// CreateUploadURLRequest is the body of POST /v1/attachments/upload-url.
type CreateUploadURLRequest struct {
	ConversationID string `json:"conversation_id"`
	Filename       string `json:"filename"`
	ContentType    string `json:"content_type"`
	Size           int64  `json:"size"`
}

// ToCommand maps to the service command.
func (r CreateUploadURLRequest) ToCommand() acontracts.CreateUploadURL {
	return acontracts.CreateUploadURL{
		ConversationID: r.ConversationID,
		Filename:       r.Filename,
		ContentType:    r.ContentType,
		Size:           r.Size,
	}
}

// UploadURLResponse is returned by the upload-url endpoint.
type UploadURLResponse struct {
	AttachmentID string            `json:"attachment_id"`
	StorageKey   string            `json:"storage_key"`
	UploadURL    string            `json:"upload_url"`
	Method       string            `json:"method"`
	Headers      map[string]string `json:"headers,omitempty"`
	ExpiresAt    time.Time         `json:"expires_at"`
}

// NewUploadURLResponse composes the response.
func NewUploadURLResponse(a *aentity.Attachment, t acontracts.UploadTarget) UploadURLResponse {
	return UploadURLResponse{
		AttachmentID: a.ID,
		StorageKey:   a.StorageKey,
		UploadURL:    t.URL,
		Method:       t.Method,
		Headers:      t.Headers,
		ExpiresAt:    t.ExpiresAt,
	}
}

// ConfirmRequest is the body of POST /v1/attachments/confirm.
type ConfirmRequest struct {
	AttachmentID string `json:"attachment_id"`
	MessageID    string `json:"message_id"`
}

// ToCommand maps to the service command.
func (r ConfirmRequest) ToCommand() acontracts.ConfirmUpload {
	return acontracts.ConfirmUpload{AttachmentID: r.AttachmentID, MessageID: r.MessageID}
}

// AttachmentResponse is the public view of an attachment.
type AttachmentResponse struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	MessageID      string    `json:"message_id,omitempty"`
	Filename       string    `json:"filename"`
	ContentType    string    `json:"content_type"`
	Size           int64     `json:"size"`
	Provider       string    `json:"storage_provider"`
	Status         string    `json:"status"`
	DownloadURL    string    `json:"download_url,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// NewAttachmentResponse maps an entity.
func NewAttachmentResponse(a *aentity.Attachment) AttachmentResponse {
	return AttachmentResponse{
		ID:             a.ID,
		ConversationID: a.ConversationID,
		MessageID:      a.MessageID,
		Filename:       a.Filename,
		ContentType:    a.ContentType,
		Size:           a.Size,
		Provider:       a.StorageProvider,
		Status:         string(a.Status),
		DownloadURL:    a.SignedURL,
		CreatedAt:      a.CreatedAt,
	}
}

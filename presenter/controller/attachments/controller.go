// Package attachments holds the HTTP controllers for the attachments endpoints:
// signed upload URL, confirm, metadata, access-checked download and the local
// blob upload sink.
package attachments

import (
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	aservice "github.com/romerito007/chat-smsnet-omnichannel/domain/attachments/service"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/attachments"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// LocalBlobStore is the local-backend sink used by the blob upload endpoint. It
// is nil when an S3-compatible backend is configured (uploads go directly to S3).
type LocalBlobStore interface {
	ResolveUpload(token string) (key, contentType string, maxSize int64, err error)
	Put(key, contentType string, data []byte) error
}

// Controller serves the attachments endpoints.
type Controller struct {
	svc  *aservice.Service
	blob LocalBlobStore
}

// NewController builds the controller. blob may be nil (S3 backend).
func NewController(svc *aservice.Service, blob LocalBlobStore) *Controller {
	return &Controller{svc: svc, blob: blob}
}

// UploadURL handles POST /v1/attachments/upload-url.
func (c *Controller) UploadURL(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateUploadURLRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	att, target, err := c.svc.RequestUploadURL(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, dto.NewUploadURLResponse(att, target))
}

// Confirm handles POST /v1/attachments/confirm.
func (c *Controller) Confirm(w http.ResponseWriter, r *http.Request) {
	var req dto.ConfirmRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	att, err := c.svc.Confirm(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewAttachmentResponse(att))
}

// Get handles GET /v1/attachments/{id}.
func (c *Controller) Get(w http.ResponseWriter, r *http.Request) {
	att, err := c.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewAttachmentResponse(att))
}

// Download handles GET /v1/attachments/{id}/download. Access to the conversation
// is always checked by the service first; for S3 the response is a 302 to a
// short-lived presigned URL, for local the bytes are streamed.
func (c *Controller) Download(w http.ResponseWriter, r *http.Request) {
	res, err := c.svc.Download(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	serveDownload(w, r, res.RedirectURL, res.ContentType, res.Filename, res.Data)
}

// ChannelMedia handles the public, JWT-less GET /v1/channel-media/{token}: it
// serves an attachment to an EXTERNAL integration system via a signed,
// time-limited token (the integration rail — no JWT, no conversation-access
// check; the signature is the credential).
func (c *Controller) ChannelMedia(w http.ResponseWriter, r *http.Request) {
	res, err := c.svc.DownloadSigned(chi.URLParam(r, "token"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	serveDownload(w, r, res.RedirectURL, res.ContentType, res.Filename, res.Data)
}

// serveDownload streams bytes or 302-redirects to a presigned URL.
func serveDownload(w http.ResponseWriter, r *http.Request, redirectURL, contentType, filename string, data []byte) {
	if redirectURL != "" {
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	if filename != "" {
		w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// BlobUpload handles PUT /v1/attachments/blobs/{token} for the local backend. The
// signed token binds the storage key, content-type and size; the body is the raw
// file. This is the "direct upload to storage" step for the local backend.
func (c *Controller) BlobUpload(w http.ResponseWriter, r *http.Request) {
	if c.blob == nil {
		middleware.WriteError(w, r, apperror.NotFound("not found"))
		return
	}
	key, contentType, maxSize, err := c.blob.ResolveUpload(chi.URLParam(r, "token"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	body := http.MaxBytesReader(w, r.Body, maxSize)
	data, err := io.ReadAll(body)
	if err != nil {
		middleware.WriteError(w, r, apperror.Validation("upload exceeds the allowed size"))
		return
	}
	if err := c.blob.Put(key, contentType, data); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

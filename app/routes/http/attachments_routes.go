package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// registerAttachmentRoutes mounts the attachments endpoints. The management
// surface is authenticated and the service enforces conversation access on every
// call (an actor may only act on attachments of a conversation it can see); the
// blob upload sink is public — the unguessable, expiring, HMAC-signed token is
// the only credential (local backend only). The conversation-access rule is
// covered by TestRequestUploadURL_EnforcesConversationAccess and
// TestDownload_ChecksAccessAndServes (domain/attachments/service).
func registerAttachmentRoutes(r chi.Router, c *container.Container) {
	ctl := factories.AttachmentController(c)

	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(c.Tokens))

		p.Post("/attachments/upload-url", ctl.UploadURL)
		p.Post("/attachments/confirm", ctl.Confirm)
		p.Get("/attachments/{id}", ctl.Get)
		p.Get("/attachments/{id}/download", ctl.Download)
	})

	// Public, signed-token blob sink for the local backend ("direct upload").
	r.Put("/attachments/blobs/{token}", ctl.BlobUpload)
}

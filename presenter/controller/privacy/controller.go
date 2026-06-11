// Package privacy holds the HTTP controllers for the privacy (LGPD) endpoints:
// contact data export, anonymization, retention configuration and the public
// signed-URL download.
package privacy

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	pcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/privacy/contracts"
	pservice "github.com/romerito007/chat-smsnet-omnichannel/domain/privacy/service"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/privacy"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves the privacy endpoints.
type Controller struct {
	svc   *pservice.Service
	files pcontracts.FileStore
}

// NewController builds the controller. files is used by the public download
// endpoint to validate the signed token and stream the export file.
func NewController(svc *pservice.Service, files pcontracts.FileStore) *Controller {
	return &Controller{svc: svc, files: files}
}

// Export handles POST /v1/privacy/contacts/{id}/export. It records the request
// and enqueues assembly, returning 202 with the pending request.
func (c *Controller) Export(w http.ResponseWriter, r *http.Request) {
	req, err := c.svc.RequestExport(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusAccepted, dto.NewExportResponse(req))
}

// GetExport handles GET /v1/privacy/exports/{id} to poll for the signed URL.
func (c *Controller) GetExport(w http.ResponseWriter, r *http.Request) {
	req, err := c.svc.GetExport(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewExportResponse(req))
}

// Anonymize handles POST /v1/privacy/contacts/{id}/anonymize.
func (c *Controller) Anonymize(w http.ResponseWriter, r *http.Request) {
	if err := c.svc.Anonymize(r.Context(), chi.URLParam(r, "id")); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetRetention handles GET /v1/privacy/retention.
func (c *Controller) GetRetention(w http.ResponseWriter, r *http.Request) {
	p, err := c.svc.GetRetention(r.Context())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewRetentionResponse(p))
}

// UpdateRetention handles PATCH /v1/privacy/retention.
func (c *Controller) UpdateRetention(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateRetentionRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	p, err := c.svc.UpdateRetention(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewRetentionResponse(p))
}

// Download handles GET /v1/privacy/downloads/{token}. Public: the unguessable,
// expiring, HMAC-signed token is the only credential (it never exposes anything
// the holder of the link was not granted), mirroring the CSAT public-token model.
func (c *Controller) Download(w http.ResponseWriter, r *http.Request) {
	key, err := c.files.Resolve(chi.URLParam(r, "token"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	data, filename, err := c.files.Open(key)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

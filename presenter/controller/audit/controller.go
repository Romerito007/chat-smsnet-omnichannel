// Package audit holds the HTTP controller for the audit-log query.
package audit

import (
	"net/http"

	arepo "github.com/romerito007/chat-smsnet-omnichannel/domain/audit/repository"
	aservice "github.com/romerito007/chat-smsnet-omnichannel/domain/audit/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/audit"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves the audit-log listing.
type Controller struct {
	svc *aservice.Service
}

// NewController builds the controller.
func NewController(svc *aservice.Service) *Controller {
	return &Controller{svc: svc}
}

// List handles GET /v1/audit, optionally filtered by ?action= (prefix),
// ?resource_id= and ?actor_id=. Gated on audit.view.
func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	page := middleware.PageFromRequest(r)
	f := arepo.Filter{
		Action:     r.URL.Query().Get("action"),
		ResourceID: r.URL.Query().Get("resource_id"),
		ActorID:    r.URL.Query().Get("actor_id"),
	}
	items, err := c.svc.List(r.Context(), f, page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	resp := shared.NewPage(dto.NewAuditLogResponses(items), page.Limit, func(a dto.AuditLogResponse) shared.Cursor {
		return shared.Cursor{CreatedAt: a.CreatedAt.UnixMilli(), ID: a.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}

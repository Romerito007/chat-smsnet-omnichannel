// Package notifications holds the HTTP controllers for the user notification
// inbox and preferences.
package notifications

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	nservice "github.com/romerito007/chat-smsnet-omnichannel/domain/notifications/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/notifications"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves the notification inbox and preferences for the
// authenticated user.
type Controller struct {
	svc *nservice.Service
}

// NewController builds the controller.
func NewController(svc *nservice.Service) *Controller {
	return &Controller{svc: svc}
}

// List handles GET /v1/notifications (optional ?unread=true).
func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	unreadOnly := r.URL.Query().Get("unread") == "true"
	page := middleware.PageFromRequest(r)
	items, err := c.svc.List(r.Context(), unreadOnly, page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	resp := shared.NewPage(dto.NewNotificationResponses(items), page.Limit, func(n dto.NotificationResponse) shared.Cursor {
		return shared.Cursor{CreatedAt: n.CreatedAt.UnixMilli(), ID: n.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}

// MarkRead handles POST /v1/notifications/{id}/read.
func (c *Controller) MarkRead(w http.ResponseWriter, r *http.Request) {
	if err := c.svc.MarkRead(r.Context(), chi.URLParam(r, "id")); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// MarkAllRead handles POST /v1/notifications/read-all.
func (c *Controller) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	n, err := c.svc.MarkAllRead(r.Context())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]any{"marked": n})
}

// GetPreferences handles GET /v1/notifications/preferences.
func (c *Controller) GetPreferences(w http.ResponseWriter, r *http.Request) {
	prefs, err := c.svc.Preferences(r.Context())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewPreferencesResponse(prefs))
}

// UpdatePreferences handles PATCH /v1/notifications/preferences.
func (c *Controller) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdatePreferencesRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	prefs, err := c.svc.UpdatePreferences(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewPreferencesResponse(prefs))
}

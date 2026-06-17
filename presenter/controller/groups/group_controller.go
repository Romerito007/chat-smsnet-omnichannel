// Package groups holds the HTTP controller for the WhatsApp groups management
// endpoints (list/search, attend toggle, gateway sync request).
package groups

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/groups/contracts"
	groupservice "github.com/romerito007/chat-smsnet-omnichannel/domain/groups/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/groups"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves the groups management reads and writes. Tenant-scoped via the
// access token.
type Controller struct {
	groups *groupservice.Service
}

// NewController builds the controller.
func NewController(groups *groupservice.Service) *Controller {
	return &Controller{groups: groups}
}

// List handles GET /v1/groups (cursor-paginated). Filters: ?q= free-text over
// name+description, ?channel_id= exact, ?attend=true|false.
func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	page := middleware.PageFromRequest(r)
	q := r.URL.Query()
	filter := contracts.ListFilter{
		Q:         q.Get("q"),
		ChannelID: q.Get("channel_id"),
	}
	switch q.Get("attend") {
	case "true":
		v := true
		filter.Attend = &v
	case "false":
		v := false
		filter.Attend = &v
	}
	items, err := c.groups.List(r.Context(), filter, page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	resp := shared.NewPage(dto.NewGroupResponses(items), page.Limit, func(it dto.GroupResponse) shared.Cursor {
		return shared.Cursor{CreatedAt: it.CreatedAt.UnixMilli(), ID: it.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}

// SetAttend handles PATCH /v1/groups/{id} (group.manage): mark a group to attend
// or not.
func (c *Controller) SetAttend(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateAttendRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	if req.Attend == nil {
		middleware.WriteError(w, r, apperror.Validation("attend is required"))
		return
	}
	g, err := c.groups.SetAttend(r.Context(), chi.URLParam(r, "id"), *req.Attend)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewGroupResponse(g))
}

// Sync handles POST /v1/groups/sync (group.manage): ask the channel's gateway to
// push its group list. Asynchronous — returns 202; the gateway pushes batches to
// the inbound groups endpoint.
func (c *Controller) Sync(w http.ResponseWriter, r *http.Request) {
	var req dto.SyncRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	if err := c.groups.Sync(r.Context(), req.ChannelID); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusAccepted, map[string]any{"ok": true})
}

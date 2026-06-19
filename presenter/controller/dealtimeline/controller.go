// Package dealtimeline holds the HTTP controller for a deal's timeline feed and the
// manual seller comments.
package dealtimeline

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	tlcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/dealtimeline/contracts"
	timelineservice "github.com/romerito007/chat-smsnet-omnichannel/domain/dealtimeline/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/dealtimeline"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves the deal-timeline reads/writes. Tenant-scoped via the token.
type Controller struct {
	timeline *timelineservice.Service
}

// NewController builds the controller.
func NewController(timeline *timelineservice.Service) *Controller {
	return &Controller{timeline: timeline}
}

// Feed handles GET /v1/deals/{id}/timeline (deal.view, deal visibility): the
// chronological feed (most recent first). Empty when the timeline module is off.
func (c *Controller) Feed(w http.ResponseWriter, r *http.Request) {
	page := middleware.PageFromRequest(r)
	items, err := c.timeline.Feed(r.Context(), chi.URLParam(r, "id"), page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	resp := shared.NewPage(items, page.Limit, func(it tlcontracts.FeedItem) shared.Cursor {
		return shared.Cursor{CreatedAt: it.CreatedAt.UnixMilli(), ID: it.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}

// Comment handles POST /v1/deals/{id}/timeline/comments (deal.manage): records a
// manual seller comment on the timeline.
func (c *Controller) Comment(w http.ResponseWriter, r *http.Request) {
	var req dto.CommentRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	item, err := c.timeline.Comment(r.Context(), chi.URLParam(r, "id"), req.TrimmedText())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, item)
}

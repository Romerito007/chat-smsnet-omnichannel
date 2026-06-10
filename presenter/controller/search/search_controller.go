// Package search holds the HTTP controllers for the search endpoints. Filters
// come from the query string; pagination is cursor-based; the service enforces
// tenant + the actor's visibility scope.
package search

import (
	"net/http"
	"time"

	scontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/search/contracts"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/search"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves the search endpoints.
type Controller struct {
	svc scontracts.SearchService
}

// NewController builds the controller.
func NewController(svc scontracts.SearchService) *Controller {
	return &Controller{svc: svc}
}

// Conversations handles GET /v1/search/conversations.
func (c *Controller) Conversations(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := scontracts.ConversationFilter{
		Status:     q.Get("status"),
		SectorID:   q.Get("sector_id"),
		AssignedTo: q.Get("assigned_to"),
		Channel:    q.Get("channel"),
		Tag:        q.Get("tag"),
		Priority:   q.Get("priority"),
		SLAStatus:  q.Get("sla_status"),
		From:       parseTime(q.Get("from")),
		To:         parseTime(q.Get("to")),
	}
	res, err := c.svc.SearchConversations(r.Context(), f, middleware.PageFromRequest(r))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewPage(dto.NewConversationHits(res.Items), res.NextCursor))
}

// Contacts handles GET /v1/search/contacts.
func (c *Controller) Contacts(w http.ResponseWriter, r *http.Request) {
	res, err := c.svc.SearchContacts(r.Context(), r.URL.Query().Get("q"), middleware.PageFromRequest(r))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewPage(dto.NewContactHits(res.Items), res.NextCursor))
}

// Messages handles GET /v1/search/messages.
func (c *Controller) Messages(w http.ResponseWriter, r *http.Request) {
	f := scontracts.MessageFilter{
		Query:          r.URL.Query().Get("q"),
		ConversationID: r.URL.Query().Get("conversation_id"),
	}
	res, err := c.svc.SearchMessages(r.Context(), f, middleware.PageFromRequest(r))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewPage(dto.NewMessageHits(res.Items), res.NextCursor))
}

// parseTime parses an RFC3339 instant, returning nil on empty/invalid input.
func parseTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}

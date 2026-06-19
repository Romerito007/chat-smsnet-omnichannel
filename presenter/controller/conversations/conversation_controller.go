// Package conversations holds the HTTP controller for the conversations
// endpoints.
package conversations

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	domaincontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	convservice "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/conversations"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves the conversations endpoints.
type Controller struct {
	svc *convservice.Service
}

// NewController builds the controller.
func NewController(svc *convservice.Service) *Controller {
	return &Controller{svc: svc}
}

// List handles GET /v1/conversations with filters + keyset pagination.
func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := domaincontracts.ListFilter{
		Status:     q.Get("status"),
		SectorID:   q.Get("sector_id"),
		QueueID:    q.Get("queue_id"),
		AssignedTo: q.Get("assigned_to"),
		ContactID:  q.Get("contact_id"),
		Protocol:   q.Get("protocol"),
		Tag:        q.Get("tag"),
	}
	page := middleware.PageFromRequest(r)
	items, err := c.svc.List(r.Context(), filter, page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	// The last-message preview is denormalized on each conversation document
	// (refreshed on every message create), so the inbox renders it straight from the
	// row — no per-page aggregation over the messages collection.
	// Resolve the contact + assignee display cards per row in two batch queries, so
	// the inbox renders each row (name/avatar) without a per-row fetch. Best-effort.
	contactCards, _ := c.svc.ContactCards(r.Context(), items)
	agentCards, _ := c.svc.AgentCards(r.Context(), items)
	resp := shared.NewPage(dto.NewConversationResponsesWithLastMessage(items, contactCards, agentCards), page.Limit, func(cv dto.ConversationResponse) shared.Cursor {
		return shared.Cursor{CreatedAt: cv.UpdatedAt.UnixMilli(), ID: cv.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}

// UnreadCounts handles GET /v1/conversations/unread-counts: the per-tab unread
// badge counts (mine / sector / queue) for the actor, in a single request.
func (c *Controller) UnreadCounts(w http.ResponseWriter, r *http.Request) {
	counts, err := c.svc.UnreadCounts(r.Context())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewUnreadCountsResponse(counts))
}

// Create handles POST /v1/conversations.
func (c *Controller) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateConversationRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	conv, err := c.svc.Create(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	single := []*entity.Conversation{conv}
	contactCards, _ := c.svc.ContactCards(r.Context(), single)
	agentCards, _ := c.svc.AgentCards(r.Context(), single)
	middleware.WriteJSON(w, http.StatusCreated, dto.NewConversationResponseWithCards(conv, contactCards, agentCards))
}

// Get handles GET /v1/conversations/{id}. The contact avatar is resolved into
// contact_avatar_url, consistent with the list.
func (c *Controller) Get(w http.ResponseWriter, r *http.Request) {
	conv, err := c.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	single := []*entity.Conversation{conv}
	contactCards, _ := c.svc.ContactCards(r.Context(), single)
	agentCards, _ := c.svc.AgentCards(r.Context(), single)
	middleware.WriteJSON(w, http.StatusOK, dto.NewConversationResponseWithCards(conv, contactCards, agentCards))
}

// Update handles PATCH /v1/conversations/{id}.
func (c *Controller) Update(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateConversationRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	conv, err := c.svc.Update(r.Context(), chi.URLParam(r, "id"), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewConversationResponse(conv))
}

// SendMessage handles POST /v1/conversations/{id}/messages.
func (c *Controller) SendMessage(w http.ResponseWriter, r *http.Request) {
	var req dto.SendMessageRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	msg, err := c.svc.SendMessage(r.Context(), chi.URLParam(r, "id"), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, dto.NewMessageResponse(msg))
}

// EditMessage handles PATCH /v1/conversations/{id}/messages/{mid}.
func (c *Controller) EditMessage(w http.ResponseWriter, r *http.Request) {
	var req dto.EditMessageRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	msg, err := c.svc.EditMessage(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "mid"), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewMessageResponse(msg))
}

// DeleteMessage handles DELETE /v1/conversations/{id}/messages/{mid}.
func (c *Controller) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	if err := c.svc.DeleteMessage(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "mid")); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// AddInternalNote handles POST /v1/conversations/{id}/internal-notes.
func (c *Controller) AddInternalNote(w http.ResponseWriter, r *http.Request) {
	var req dto.InternalNoteRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	msg, err := c.svc.AddInternalNote(r.Context(), chi.URLParam(r, "id"), domaincontracts.AddInternalNote{Text: req.Text, MentionUserIDs: req.MentionUserIDs})
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, dto.NewMessageResponse(msg))
}

// Close handles POST /v1/conversations/{id}/close.
func (c *Controller) Close(w http.ResponseWriter, r *http.Request) {
	var req dto.CloseRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	conv, err := c.svc.Close(r.Context(), chi.URLParam(r, "id"), domaincontracts.CloseConversation{
		CloseReasonID: req.CloseReasonID,
		Note:          req.Note,
	})
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewConversationResponse(conv))
}

// Reopen handles POST /v1/conversations/{id}/reopen.
func (c *Controller) Reopen(w http.ResponseWriter, r *http.Request) {
	conv, err := c.svc.Reopen(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewConversationResponse(conv))
}

// TypingStart handles POST /v1/conversations/{id}/typing/start.
func (c *Controller) TypingStart(w http.ResponseWriter, r *http.Request) {
	if err := c.svc.SetTyping(r.Context(), chi.URLParam(r, "id"), true); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// TypingStop handles POST /v1/conversations/{id}/typing/stop.
func (c *Controller) TypingStop(w http.ResponseWriter, r *http.Request) {
	if err := c.svc.SetTyping(r.Context(), chi.URLParam(r, "id"), false); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Read handles POST /v1/conversations/{id}/read.
func (c *Controller) Read(w http.ResponseWriter, r *http.Request) {
	if err := c.svc.MarkRead(r.Context(), chi.URLParam(r, "id")); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Unread handles POST /v1/conversations/{id}/unread: re-lights the unread dot
// (unread_count=1) when the conversation is currently read; a no-op when it
// already has unread messages.
func (c *Controller) Unread(w http.ResponseWriter, r *http.Request) {
	if err := c.svc.MarkUnread(r.Context(), chi.URLParam(r, "id")); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListMessages handles GET /v1/conversations/{id}/messages.
func (c *Controller) ListMessages(w http.ResponseWriter, r *http.Request) {
	page := middleware.PageFromRequest(r)
	items, err := c.svc.ListMessages(r.Context(), chi.URLParam(r, "id"), page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	resp := shared.NewPage(dto.NewMessageResponses(items), page.Limit, func(m dto.MessageResponse) shared.Cursor {
		return shared.Cursor{CreatedAt: m.CreatedAt.UnixMilli(), ID: m.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}

// ListEvents handles GET /v1/conversations/{id}/events — the lifecycle/automation
// timeline, persisted separately from chat messages.
func (c *Controller) ListEvents(w http.ResponseWriter, r *http.Request) {
	page := middleware.PageFromRequest(r)
	items, err := c.svc.ListEvents(r.Context(), chi.URLParam(r, "id"), page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	resp := shared.NewPage(dto.NewEventResponses(items), page.Limit, func(e dto.EventResponse) shared.Cursor {
		return shared.Cursor{CreatedAt: e.CreatedAt.UnixMilli(), ID: e.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}

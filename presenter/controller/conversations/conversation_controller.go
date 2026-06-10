// Package conversations holds the HTTP controller for the conversations
// endpoints.
package conversations

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	domaincontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
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
		Tag:        q.Get("tag"),
	}
	page := middleware.PageFromRequest(r)
	items, err := c.svc.List(r.Context(), filter, page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	resp := shared.NewPage(dto.NewConversationResponses(items), page.Limit, func(cv dto.ConversationResponse) shared.Cursor {
		return shared.Cursor{CreatedAt: cv.UpdatedAt.UnixMilli(), ID: cv.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
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
	middleware.WriteJSON(w, http.StatusCreated, dto.NewConversationResponse(conv))
}

// Get handles GET /v1/conversations/{id}.
func (c *Controller) Get(w http.ResponseWriter, r *http.Request) {
	conv, err := c.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewConversationResponse(conv))
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

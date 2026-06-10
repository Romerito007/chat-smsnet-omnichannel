// Package conversationtools holds the HTTP controllers for tags, canned
// responses, close reasons and the conversation tag-apply endpoint.
package conversationtools

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	convservice "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/service"
	ctservice "github.com/romerito007/chat-smsnet-omnichannel/domain/conversationtools/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	convdto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/conversations"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/conversationtools"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves tag, canned-response and close-reason CRUD plus the
// conversation tag-apply endpoint.
type Controller struct {
	tags          *ctservice.TagService
	canned        *ctservice.CannedResponseService
	closeReasons  *ctservice.CloseReasonService
	conversations *convservice.Service
}

// NewController builds the controller.
func NewController(
	tags *ctservice.TagService,
	canned *ctservice.CannedResponseService,
	closeReasons *ctservice.CloseReasonService,
	conversations *convservice.Service,
) *Controller {
	return &Controller{tags: tags, canned: canned, closeReasons: closeReasons, conversations: conversations}
}

// ── tags ─────────────────────────────────────────────────────────────────────

func (c *Controller) ListTags(w http.ResponseWriter, r *http.Request) {
	page := middleware.PageFromRequest(r)
	items, err := c.tags.List(r.Context(), page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	resp := shared.NewPage(dto.NewTagResponses(items), page.Limit, func(t dto.TagResponse) shared.Cursor {
		return shared.Cursor{CreatedAt: t.CreatedAt.UnixMilli(), ID: t.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}

func (c *Controller) CreateTag(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateTagRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	t, err := c.tags.Create(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, dto.NewTagResponse(t))
}

func (c *Controller) GetTag(w http.ResponseWriter, r *http.Request) {
	t, err := c.tags.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewTagResponse(t))
}

func (c *Controller) UpdateTag(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateTagRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	t, err := c.tags.Update(r.Context(), chi.URLParam(r, "id"), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewTagResponse(t))
}

func (c *Controller) DeleteTag(w http.ResponseWriter, r *http.Request) {
	if err := c.tags.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── canned responses ─────────────────────────────────────────────────────────

func (c *Controller) ListCanned(w http.ResponseWriter, r *http.Request) {
	// A shortcut query resolves a single canned response (the "type a shortcut,
	// get the body" lookup), scoped to the actor's sectors.
	if shortcut := r.URL.Query().Get("shortcut"); shortcut != "" {
		cr, err := c.canned.Resolve(r.Context(), shortcut)
		if err != nil {
			middleware.WriteError(w, r, err)
			return
		}
		middleware.WriteJSON(w, http.StatusOK, dto.NewCannedResponse(cr))
		return
	}
	page := middleware.PageFromRequest(r)
	items, err := c.canned.List(r.Context(), page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	resp := shared.NewPage(dto.NewCannedResponses(items), page.Limit, func(x dto.CannedResponse) shared.Cursor {
		return shared.Cursor{CreatedAt: x.CreatedAt.UnixMilli(), ID: x.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}

func (c *Controller) CreateCanned(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateCannedRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	cr, err := c.canned.Create(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, dto.NewCannedResponse(cr))
}

func (c *Controller) GetCanned(w http.ResponseWriter, r *http.Request) {
	cr, err := c.canned.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewCannedResponse(cr))
}

func (c *Controller) UpdateCanned(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateCannedRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	cr, err := c.canned.Update(r.Context(), chi.URLParam(r, "id"), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewCannedResponse(cr))
}

func (c *Controller) DeleteCanned(w http.ResponseWriter, r *http.Request) {
	if err := c.canned.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── close reasons ────────────────────────────────────────────────────────────

func (c *Controller) ListCloseReasons(w http.ResponseWriter, r *http.Request) {
	page := middleware.PageFromRequest(r)
	items, err := c.closeReasons.List(r.Context(), page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	resp := shared.NewPage(dto.NewCloseReasonResponses(items), page.Limit, func(x dto.CloseReasonResponse) shared.Cursor {
		return shared.Cursor{CreatedAt: x.CreatedAt.UnixMilli(), ID: x.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}

func (c *Controller) CreateCloseReason(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateCloseReasonRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	cr, err := c.closeReasons.Create(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, dto.NewCloseReasonResponse(cr))
}

func (c *Controller) GetCloseReason(w http.ResponseWriter, r *http.Request) {
	cr, err := c.closeReasons.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewCloseReasonResponse(cr))
}

func (c *Controller) UpdateCloseReason(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateCloseReasonRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	cr, err := c.closeReasons.Update(r.Context(), chi.URLParam(r, "id"), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewCloseReasonResponse(cr))
}

func (c *Controller) DeleteCloseReason(w http.ResponseWriter, r *http.Request) {
	if err := c.closeReasons.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── apply tags to a conversation ─────────────────────────────────────────────

// ApplyTags handles POST /v1/conversations/{id}/tags. It delegates to the
// conversations service, which validates the tags against the catalog, records
// a conversation.tagged event and publishes the realtime update.
func (c *Controller) ApplyTags(w http.ResponseWriter, r *http.Request) {
	var req dto.ApplyTagsRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	conv, err := c.conversations.ApplyTags(r.Context(), chi.URLParam(r, "id"), req.Add, req.Remove)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, convdto.NewConversationResponse(conv))
}

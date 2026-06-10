// Package csat holds the HTTP controllers for CSAT survey CRUD, the responses
// listing and the public token answer.
package csat

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	cservice "github.com/romerito007/chat-smsnet-omnichannel/domain/csat/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/csat"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves survey CRUD, responses listing and the public answer.
type Controller struct {
	surveys *cservice.SurveyService
	flow    *cservice.Service
}

// NewController builds the controller.
func NewController(surveys *cservice.SurveyService, flow *cservice.Service) *Controller {
	return &Controller{surveys: surveys, flow: flow}
}

// ── surveys ──────────────────────────────────────────────────────────────────

func (c *Controller) ListSurveys(w http.ResponseWriter, r *http.Request) {
	page := middleware.PageFromRequest(r)
	items, err := c.surveys.List(r.Context(), page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	resp := shared.NewPage(dto.NewSurveyResponses(items), page.Limit, func(s dto.SurveyResponse) shared.Cursor {
		return shared.Cursor{CreatedAt: s.CreatedAt.UnixMilli(), ID: s.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}

func (c *Controller) CreateSurvey(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateSurveyRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	s, err := c.surveys.Create(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, dto.NewSurveyResponse(s))
}

func (c *Controller) GetSurvey(w http.ResponseWriter, r *http.Request) {
	s, err := c.surveys.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewSurveyResponse(s))
}

func (c *Controller) UpdateSurvey(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateSurveyRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	s, err := c.surveys.Update(r.Context(), chi.URLParam(r, "id"), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewSurveyResponse(s))
}

func (c *Controller) DeleteSurvey(w http.ResponseWriter, r *http.Request) {
	if err := c.surveys.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── responses ────────────────────────────────────────────────────────────────

// ListResponses handles GET /v1/csat/responses (reporting).
func (c *Controller) ListResponses(w http.ResponseWriter, r *http.Request) {
	page := middleware.PageFromRequest(r)
	items, err := c.flow.ListResponses(r.Context(), page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	resp := shared.NewPage(dto.NewResponseResponses(items), page.Limit, func(x dto.ResponseResponse) shared.Cursor {
		return shared.Cursor{CreatedAt: x.CreatedAt.UnixMilli(), ID: x.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}

// Submit handles the public POST /v1/csat/responses/{token}. It validates only
// the token and the score; it never exposes the conversation.
func (c *Controller) Submit(w http.ResponseWriter, r *http.Request) {
	var req dto.SubmitRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	if err := c.flow.SubmitByToken(r.Context(), chi.URLParam(r, "token"), req.ToCommand()); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]any{"status": "recorded", "message": "Thank you for your feedback!"})
}

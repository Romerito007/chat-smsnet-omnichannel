// Package csat holds the HTTP controllers for CSAT survey CRUD, the responses
// listing and the public token answer.
package csat

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	cservice "github.com/romerito007/chat-smsnet-omnichannel/domain/csat/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/csat"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// ContactDirectory resolves contact ids to display cards (name + avatar) so the CSAT
// report renders the contact name instead of a raw id. Satisfied by the contacts
// service (ContactCards). Optional.
type ContactDirectory interface {
	ContactCards(ctx context.Context, contactIDs []string) (map[string]shared.DisplayCard, error)
}

// AgentDirectory resolves agent ids to display cards (name + signed avatar URL),
// reusing the IAM user service (AgentCards) — same source the reports use. Optional.
type AgentDirectory interface {
	AgentCards(ctx context.Context, userIDs []string) (map[string]shared.DisplayCard, error)
}

// SurveyDirectory resolves survey ids to names in batch. Satisfied by the survey
// service (SurveyNames). Optional.
type SurveyDirectory interface {
	SurveyNames(ctx context.Context, ids []string) (map[string]string, error)
}

// Controller serves survey CRUD, responses listing and the public answer.
type Controller struct {
	surveys     *cservice.SurveyService
	flow        *cservice.Service
	contactsDir ContactDirectory
	agentsDir   AgentDirectory
	surveysDir  SurveyDirectory
}

// NewController builds the controller.
func NewController(surveys *cservice.SurveyService, flow *cservice.Service) *Controller {
	return &Controller{surveys: surveys, flow: flow}
}

// SetDirectories wires the contact/agent/survey directories used to enrich the CSAT
// responses report with display names (so the dashboard never shows a raw id).
// Optional: when unset, rows carry only the raw ids.
func (c *Controller) SetDirectories(contacts ContactDirectory, agents AgentDirectory, surveys SurveyDirectory) *Controller {
	c.contactsDir = contacts
	c.agentsDir = agents
	c.surveysDir = surveys
	return c
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

// ListResponses handles GET /v1/csat/responses (reporting). The raw contact/agent/
// survey ids on each row are enriched with the resolved names (+ agent avatar),
// resolved in batch so the dashboard never renders a bare id.
func (c *Controller) ListResponses(w http.ResponseWriter, r *http.Request) {
	page := middleware.PageFromRequest(r)
	items, err := c.flow.ListResponses(r.Context(), page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	rows := dto.NewResponseResponses(items)
	c.enrichResponses(r.Context(), rows)
	resp := shared.NewPage(rows, page.Limit, func(x dto.ResponseResponse) shared.Cursor {
		return shared.Cursor{CreatedAt: x.CreatedAt.UnixMilli(), ID: x.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}

// enrichResponses fills the contact/agent/survey display names (+ agent avatar) on
// each row, each resolved in ONE batch call. Best-effort: a missing directory or a
// lookup error leaves the rows with their raw ids rather than failing the listing.
func (c *Controller) enrichResponses(ctx context.Context, rows []dto.ResponseResponse) {
	if len(rows) == 0 {
		return
	}
	if c.contactsDir != nil {
		ids := collectIDs(rows, func(x dto.ResponseResponse) string { return x.ContactID })
		if cards, err := c.contactsDir.ContactCards(ctx, ids); err == nil {
			for i := range rows {
				if card, ok := cards[rows[i].ContactID]; ok {
					rows[i].ContactName = card.Name
				}
			}
		}
	}
	if c.agentsDir != nil {
		ids := collectIDs(rows, func(x dto.ResponseResponse) string { return x.AgentID })
		if cards, err := c.agentsDir.AgentCards(ctx, ids); err == nil {
			for i := range rows {
				if card, ok := cards[rows[i].AgentID]; ok {
					rows[i].AgentName = card.Name
					rows[i].AgentAvatarURL = card.AvatarURL
				}
			}
		}
	}
	if c.surveysDir != nil {
		ids := collectIDs(rows, func(x dto.ResponseResponse) string { return x.SurveyID })
		if names, err := c.surveysDir.SurveyNames(ctx, ids); err == nil {
			for i := range rows {
				if name, ok := names[rows[i].SurveyID]; ok {
					rows[i].SurveyName = name
				}
			}
		}
	}
}

// collectIDs gathers the distinct non-empty ids selected from the rows.
func collectIDs(rows []dto.ResponseResponse, pick func(dto.ResponseResponse) string) []string {
	seen := make(map[string]struct{}, len(rows))
	ids := make([]string, 0, len(rows))
	for _, x := range rows {
		id := pick(x)
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
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

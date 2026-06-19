// Package deals holds the HTTP controller for the sales-deal (Kanban card) endpoints.
package deals

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	dcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/deals/contracts"
	dealentity "github.com/romerito007/chat-smsnet-omnichannel/domain/deals/entity"
	dealservice "github.com/romerito007/chat-smsnet-omnichannel/domain/deals/service"
	pipelineentity "github.com/romerito007/chat-smsnet-omnichannel/domain/pipelines/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/deals"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// ContactDirectory resolves contact ids to display cards (contact_name). Optional.
type ContactDirectory interface {
	ContactCards(ctx context.Context, contactIDs []string) (map[string]shared.DisplayCard, error)
}

// AgentDirectory resolves agent ids to display cards (name + avatar). Optional.
type AgentDirectory interface {
	AgentCards(ctx context.Context, userIDs []string) (map[string]shared.DisplayCard, error)
}

// PipelineDirectory lists the tenant's pipelines so the controller can resolve
// pipeline_name and stage_name in one call. Optional.
type PipelineDirectory interface {
	List(ctx context.Context) ([]*pipelineentity.Pipeline, error)
}

// Controller serves deal reads and writes. Tenant-scoped via the token.
type Controller struct {
	deals        *dealservice.Service
	contactsDir  ContactDirectory
	agentsDir    AgentDirectory
	pipelinesDir PipelineDirectory
}

// NewController builds the controller.
func NewController(deals *dealservice.Service) *Controller {
	return &Controller{deals: deals}
}

// SetDirectories wires the contact/agent/pipeline directories used to enrich the
// deal responses with names (so the Kanban never shows raw ids). Optional.
func (c *Controller) SetDirectories(contacts ContactDirectory, agents AgentDirectory, pipelines PipelineDirectory) *Controller {
	c.contactsDir = contacts
	c.agentsDir = agents
	c.pipelinesDir = pipelines
	return c
}

// List handles GET /v1/deals (deal.view). Filters: pipeline_id, stage_id,
// assigned_to, status, q (title); the Kanban consumes this and groups by stage.
func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	page := middleware.PageFromRequest(r)
	q := r.URL.Query()
	filter := dcontracts.ListFilter{
		PipelineID: q.Get("pipeline_id"),
		StageID:    q.Get("stage_id"),
		AssignedTo: q.Get("assigned_to"),
		ContactID:  q.Get("contact_id"),
		Status:     q.Get("status"),
		Q:          q.Get("q"),
	}
	items, err := c.deals.List(r.Context(), filter, page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	rows := dto.NewDealResponses(items)
	c.enrich(r.Context(), rows)
	resp := shared.NewPage(rows, page.Limit, func(d dto.DealResponse) shared.Cursor {
		return shared.Cursor{CreatedAt: d.CreatedAt.UnixMilli(), ID: d.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}

// Get handles GET /v1/deals/{id} (deal.view).
func (c *Controller) Get(w http.ResponseWriter, r *http.Request) {
	d, err := c.deals.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	c.writeEnriched(w, r, d)
}

// Create handles POST /v1/deals (deal.manage).
func (c *Controller) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateDealRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	d, err := c.deals.Create(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	c.writeEnrichedStatus(w, r, d, http.StatusCreated)
}

// CreateFromConversation handles POST /v1/deals/from-conversation (deal.manage).
func (c *Controller) CreateFromConversation(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateFromConversationRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	d, err := c.deals.CreateFromConversation(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	c.writeEnrichedStatus(w, r, d, http.StatusCreated)
}

// Update handles PATCH /v1/deals/{id} (deal.manage).
func (c *Controller) Update(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateDealRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	d, err := c.deals.Update(r.Context(), chi.URLParam(r, "id"), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	c.writeEnriched(w, r, d)
}

// MoveStage handles PATCH /v1/deals/{id}/stage (deal.manage) — the drag-and-drop.
func (c *Controller) MoveStage(w http.ResponseWriter, r *http.Request) {
	var req dto.MoveStageRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	d, err := c.deals.MoveStage(r.Context(), chi.URLParam(r, "id"), req.StageID)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	c.writeEnriched(w, r, d)
}

// LinkConversation handles POST /v1/deals/{id}/conversations (deal.manage).
func (c *Controller) LinkConversation(w http.ResponseWriter, r *http.Request) {
	var req dto.LinkConversationRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	d, err := c.deals.LinkConversation(r.Context(), chi.URLParam(r, "id"), req.ConversationID)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	c.writeEnriched(w, r, d)
}

// MarkLost handles POST /v1/deals/{id}/lost (deal.manage).
func (c *Controller) MarkLost(w http.ResponseWriter, r *http.Request) {
	var req dto.MarkLostRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	d, err := c.deals.MarkLost(r.Context(), chi.URLParam(r, "id"), req.Reason)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	c.writeEnriched(w, r, d)
}

// AddItem handles POST /v1/deals/{id}/items (deal.manage): snapshot a product onto
// the deal and recompute the value.
func (c *Controller) AddItem(w http.ResponseWriter, r *http.Request) {
	var req dto.AddItemRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	d, err := c.deals.AddItem(r.Context(), chi.URLParam(r, "id"), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	c.writeEnrichedStatus(w, r, d, http.StatusCreated)
}

// UpdateItem handles PATCH /v1/deals/{id}/items/{itemId} (deal.manage).
func (c *Controller) UpdateItem(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateItemRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	d, err := c.deals.UpdateItem(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "itemId"), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	c.writeEnriched(w, r, d)
}

// RemoveItem handles DELETE /v1/deals/{id}/items/{itemId} (deal.manage).
func (c *Controller) RemoveItem(w http.ResponseWriter, r *http.Request) {
	d, err := c.deals.RemoveItem(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "itemId"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	c.writeEnriched(w, r, d)
}

// Delete handles DELETE /v1/deals/{id} (deal.manage).
func (c *Controller) Delete(w http.ResponseWriter, r *http.Request) {
	if err := c.deals.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── enrichment ───────────────────────────────────────────────────────────────

func (c *Controller) writeEnriched(w http.ResponseWriter, r *http.Request, d *dealentity.Deal) {
	c.writeEnrichedStatus(w, r, d, http.StatusOK)
}

func (c *Controller) writeEnrichedStatus(w http.ResponseWriter, r *http.Request, d *dealentity.Deal, status int) {
	rows := []dto.DealResponse{dto.NewDealResponse(d)}
	c.enrich(r.Context(), rows)
	middleware.WriteJSON(w, status, rows[0])
}

// enrich resolves the contact/agent/pipeline names on a page of deals, each in ONE
// batch call. Best-effort: a missing directory or lookup error leaves rows with
// their raw ids.
func (c *Controller) enrich(ctx context.Context, rows []dto.DealResponse) {
	if len(rows) == 0 {
		return
	}
	if c.contactsDir != nil {
		ids := distinct(rows, func(d dto.DealResponse) string { return d.ContactID })
		if cards, err := c.contactsDir.ContactCards(ctx, ids); err == nil {
			for i := range rows {
				if card, ok := cards[rows[i].ContactID]; ok {
					rows[i].ContactName = card.Name
				}
			}
		}
	}
	if c.agentsDir != nil {
		ids := distinct(rows, func(d dto.DealResponse) string { return d.AssignedTo })
		if cards, err := c.agentsDir.AgentCards(ctx, ids); err == nil {
			for i := range rows {
				if card, ok := cards[rows[i].AssignedTo]; ok {
					rows[i].AssignedToName = card.Name
					rows[i].AssignedToAvatarURL = card.AvatarURL
				}
			}
		}
	}
	if c.pipelinesDir != nil {
		c.labelPipelines(ctx, rows)
	}
}

// labelPipelines resolves pipeline_name + stage_name from the tenant's pipelines
// (loaded once — pipelines are few per tenant).
func (c *Controller) labelPipelines(ctx context.Context, rows []dto.DealResponse) {
	pls, err := c.pipelinesDir.List(ctx)
	if err != nil {
		return
	}
	pname := make(map[string]string, len(pls))
	sname := make(map[string]map[string]string, len(pls))
	for _, p := range pls {
		pname[p.ID] = p.Name
		stages := make(map[string]string, len(p.Stages))
		for _, st := range p.Stages {
			stages[st.ID] = st.Name
		}
		sname[p.ID] = stages
	}
	for i := range rows {
		if n, ok := pname[rows[i].PipelineID]; ok {
			rows[i].PipelineName = n
		}
		if stages, ok := sname[rows[i].PipelineID]; ok {
			if n, ok := stages[rows[i].StageID]; ok {
				rows[i].StageName = n
			}
		}
	}
}

func distinct(rows []dto.DealResponse, pick func(dto.DealResponse) string) []string {
	seen := make(map[string]struct{}, len(rows))
	ids := make([]string, 0, len(rows))
	for _, d := range rows {
		id := pick(d)
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

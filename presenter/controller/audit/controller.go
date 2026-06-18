// Package audit holds the HTTP controller for the audit-log query.
package audit

import (
	"context"
	"net/http"

	arepo "github.com/romerito007/chat-smsnet-omnichannel/domain/audit/repository"
	aservice "github.com/romerito007/chat-smsnet-omnichannel/domain/audit/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/audit"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// AgentDirectory resolves actor (user) ids to display cards so the log renders the
// actor name instead of a raw id. Satisfied by the IAM user service (AgentCards) —
// the same source the reports use. Optional.
type AgentDirectory interface {
	AgentCards(ctx context.Context, userIDs []string) (map[string]shared.DisplayCard, error)
}

// Controller serves the audit-log listing.
type Controller struct {
	svc    *aservice.Service
	agents AgentDirectory
}

// NewController builds the controller.
func NewController(svc *aservice.Service) *Controller {
	return &Controller{svc: svc}
}

// SetDirectories wires the agent directory used to resolve actor_id → actor_name.
// Optional: when unset, rows carry only the raw actor id.
func (c *Controller) SetDirectories(agents AgentDirectory) *Controller {
	c.agents = agents
	return c
}

// List handles GET /v1/audit, optionally filtered by ?action= (prefix),
// ?resource_id= and ?actor_id=. Gated on audit.view.
func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	page := middleware.PageFromRequest(r)
	f := arepo.Filter{
		Action:     r.URL.Query().Get("action"),
		ResourceID: r.URL.Query().Get("resource_id"),
		ActorID:    r.URL.Query().Get("actor_id"),
	}
	items, err := c.svc.List(r.Context(), f, page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	rows := dto.NewAuditLogResponses(items)
	c.enrichActors(r.Context(), rows)
	resp := shared.NewPage(rows, page.Limit, func(a dto.AuditLogResponse) shared.Cursor {
		return shared.Cursor{CreatedAt: a.CreatedAt.UnixMilli(), ID: a.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}

// enrichActors fills actor_name from the resolved user card, in ONE batch call.
// Best-effort: a missing directory or a lookup error leaves rows with only the raw
// actor id. Non-user actors (system/platform) simply don't resolve.
func (c *Controller) enrichActors(ctx context.Context, rows []dto.AuditLogResponse) {
	if c.agents == nil || len(rows) == 0 {
		return
	}
	seen := make(map[string]struct{}, len(rows))
	ids := make([]string, 0, len(rows))
	for _, a := range rows {
		if a.ActorID == "" {
			continue
		}
		if _, dup := seen[a.ActorID]; dup {
			continue
		}
		seen[a.ActorID] = struct{}{}
		ids = append(ids, a.ActorID)
	}
	cards, err := c.agents.AgentCards(ctx, ids)
	if err != nil {
		return
	}
	for i := range rows {
		if card, ok := cards[rows[i].ActorID]; ok {
			rows[i].ActorName = card.Name
		}
	}
}

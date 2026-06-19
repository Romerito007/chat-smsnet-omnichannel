// Package dealtasks holds the HTTP controller for the deal-task (seller follow-up)
// endpoints — per-deal and the consolidated "my tasks" view.
package dealtasks

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	tcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/dealtasks/contracts"
	taskservice "github.com/romerito007/chat-smsnet-omnichannel/domain/dealtasks/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/dealtasks"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves the deal-task reads/writes. Tenant-scoped via the token.
type Controller struct {
	tasks *taskservice.Service
}

// NewController builds the controller.
func NewController(tasks *taskservice.Service) *Controller {
	return &Controller{tasks: tasks}
}

// ListByDeal handles GET /v1/deals/{id}/tasks (deal.view). Empty when the tasks
// module is off.
func (c *Controller) ListByDeal(w http.ResponseWriter, r *http.Request) {
	page := middleware.PageFromRequest(r)
	items, err := c.tasks.ListByDeal(r.Context(), chi.URLParam(r, "id"), page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	c.writePage(w, items, page)
}

// Create handles POST /v1/deals/{id}/tasks (deal.manage).
func (c *Controller) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateTaskRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	t, err := c.tasks.Create(r.Context(), req.ToCommand(chi.URLParam(r, "id")))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, t)
}

// Update handles PATCH /v1/deals/{id}/tasks/{taskId} (deal.manage).
func (c *Controller) Update(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateTaskRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	t, err := c.tasks.Update(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "taskId"), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, t)
}

// Complete handles POST /v1/deals/{id}/tasks/{taskId}/complete (deal.manage).
func (c *Controller) Complete(w http.ResponseWriter, r *http.Request) {
	t, err := c.tasks.Complete(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "taskId"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, t)
}

// Delete handles DELETE /v1/deals/{id}/tasks/{taskId} (deal.manage).
func (c *Controller) Delete(w http.ResponseWriter, r *http.Request) {
	if err := c.tasks.Delete(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "taskId")); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListMine handles GET /v1/crm/tasks?assigned_to=&status=&due_before= (deal.view):
// the consolidated seller task board. A non-all-scope actor sees only its own tasks.
func (c *Controller) ListMine(w http.ResponseWriter, r *http.Request) {
	page := middleware.PageFromRequest(r)
	q := r.URL.Query()
	filter := tcontracts.ListFilter{AssignedTo: q.Get("assigned_to"), Status: q.Get("status")}
	if v := q.Get("due_before"); v != "" {
		if ts, err := time.Parse(time.RFC3339, v); err == nil {
			filter.DueBefore = &ts
		}
	}
	items, err := c.tasks.ListMine(r.Context(), filter, page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	c.writePage(w, items, page)
}

func (c *Controller) writePage(w http.ResponseWriter, items []tcontracts.TaskView, page shared.PageRequest) {
	resp := shared.NewPage(items, page.Limit, func(it tcontracts.TaskView) shared.Cursor {
		return shared.Cursor{CreatedAt: it.CreatedAt.UnixMilli(), ID: it.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}

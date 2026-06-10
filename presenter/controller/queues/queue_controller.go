// Package queues holds the HTTP controller for the queue endpoints.
package queues

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	queueservice "github.com/romerito007/chat-smsnet-omnichannel/domain/queues/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/queues"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves CRUD for queues.
type Controller struct {
	queues *queueservice.Service
}

// NewController builds the controller.
func NewController(queues *queueservice.Service) *Controller {
	return &Controller{queues: queues}
}

// Create handles POST /v1/queues.
func (c *Controller) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateQueueRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	q, err := c.queues.Create(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, dto.NewQueueResponse(q))
}

// List handles GET /v1/queues.
func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	page := middleware.PageFromRequest(r)
	items, err := c.queues.List(r.Context(), page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	resp := shared.NewPage(dto.NewQueueResponses(items), page.Limit, func(q dto.QueueResponse) shared.Cursor {
		return shared.Cursor{CreatedAt: q.CreatedAt.UnixMilli(), ID: q.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}

// Get handles GET /v1/queues/{id}.
func (c *Controller) Get(w http.ResponseWriter, r *http.Request) {
	q, err := c.queues.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewQueueResponse(q))
}

// Update handles PATCH /v1/queues/{id}.
func (c *Controller) Update(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateQueueRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	q, err := c.queues.Update(r.Context(), chi.URLParam(r, "id"), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewQueueResponse(q))
}

// Delete handles DELETE /v1/queues/{id}.
func (c *Controller) Delete(w http.ResponseWriter, r *http.Request) {
	if err := c.queues.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

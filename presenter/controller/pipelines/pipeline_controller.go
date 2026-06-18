// Package pipelines holds the HTTP controller for the sales-pipeline (Kanban funnel)
// management endpoints.
package pipelines

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	pipelineservice "github.com/romerito007/chat-smsnet-omnichannel/domain/pipelines/service"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/pipelines"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves the pipeline reads and writes. Tenant-scoped via the token.
type Controller struct {
	pipelines *pipelineservice.Service
}

// NewController builds the controller.
func NewController(pipelines *pipelineservice.Service) *Controller {
	return &Controller{pipelines: pipelines}
}

// List handles GET /v1/pipelines (pipeline.view).
func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	items, err := c.pipelines.List(r.Context())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]any{"data": dto.NewPipelineResponses(items)})
}

// Get handles GET /v1/pipelines/{id} (pipeline.view).
func (c *Controller) Get(w http.ResponseWriter, r *http.Request) {
	p, err := c.pipelines.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewPipelineResponse(p))
}

// Create handles POST /v1/pipelines (pipeline.manage).
func (c *Controller) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreatePipelineRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	p, err := c.pipelines.Create(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, dto.NewPipelineResponse(p))
}

// Update handles PATCH /v1/pipelines/{id} (pipeline.manage): rename / set default.
func (c *Controller) Update(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdatePipelineRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	p, err := c.pipelines.Update(r.Context(), chi.URLParam(r, "id"), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewPipelineResponse(p))
}

// Delete handles DELETE /v1/pipelines/{id} (pipeline.manage).
func (c *Controller) Delete(w http.ResponseWriter, r *http.Request) {
	if err := c.pipelines.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// AddStage handles POST /v1/pipelines/{id}/stages (pipeline.manage).
func (c *Controller) AddStage(w http.ResponseWriter, r *http.Request) {
	var req dto.AddStageRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	p, err := c.pipelines.AddStage(r.Context(), chi.URLParam(r, "id"), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, dto.NewPipelineResponse(p))
}

// UpdateStage handles PATCH /v1/pipelines/{id}/stages/{stageId} (pipeline.manage).
func (c *Controller) UpdateStage(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateStageRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	p, err := c.pipelines.UpdateStage(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "stageId"), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewPipelineResponse(p))
}

// ReorderStages handles POST /v1/pipelines/{id}/stages/reorder (pipeline.manage).
func (c *Controller) ReorderStages(w http.ResponseWriter, r *http.Request) {
	var req dto.ReorderStagesRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	p, err := c.pipelines.ReorderStages(r.Context(), chi.URLParam(r, "id"), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewPipelineResponse(p))
}

// DeleteStage handles DELETE /v1/pipelines/{id}/stages/{stageId} (pipeline.manage).
func (c *Controller) DeleteStage(w http.ResponseWriter, r *http.Request) {
	p, err := c.pipelines.DeleteStage(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "stageId"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewPipelineResponse(p))
}

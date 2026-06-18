// Package pipelines holds the request/response DTOs for the sales-pipeline
// (Kanban funnel) endpoints.
package pipelines

import (
	"time"

	ccontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/pipelines/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/pipelines/entity"
)

// ── requests ─────────────────────────────────────────────────────────────────

// StageRequest is one stage in a create-pipeline body.
type StageRequest struct {
	Name   string `json:"name"`
	Order  int    `json:"order"`
	IsWon  bool   `json:"is_won"`
	IsLost bool   `json:"is_lost"`
	Color  string `json:"color"`
}

// CreatePipelineRequest is the body of POST /v1/pipelines.
type CreatePipelineRequest struct {
	Name   string         `json:"name"`
	Stages []StageRequest `json:"stages"`
}

// ToCommand maps the request to the service command.
func (r CreatePipelineRequest) ToCommand() ccontracts.CreatePipeline {
	stages := make([]ccontracts.StageInput, 0, len(r.Stages))
	for _, st := range r.Stages {
		stages = append(stages, ccontracts.StageInput{
			Name: st.Name, Order: st.Order, IsWon: st.IsWon, IsLost: st.IsLost, Color: st.Color,
		})
	}
	return ccontracts.CreatePipeline{Name: r.Name, Stages: stages}
}

// UpdatePipelineRequest is the body of PATCH /v1/pipelines/{id}. Nil = unchanged.
type UpdatePipelineRequest struct {
	Name      *string `json:"name"`
	IsDefault *bool   `json:"is_default"`
}

// ToCommand maps to the service command.
func (r UpdatePipelineRequest) ToCommand() ccontracts.UpdatePipeline {
	return ccontracts.UpdatePipeline{Name: r.Name, IsDefault: r.IsDefault}
}

// AddStageRequest is the body of POST /v1/pipelines/{id}/stages.
type AddStageRequest struct {
	Name   string `json:"name"`
	Order  int    `json:"order"`
	IsWon  bool   `json:"is_won"`
	IsLost bool   `json:"is_lost"`
	Color  string `json:"color"`
}

// ToCommand maps to the service command.
func (r AddStageRequest) ToCommand() ccontracts.AddStage {
	return ccontracts.AddStage{Name: r.Name, Order: r.Order, IsWon: r.IsWon, IsLost: r.IsLost, Color: r.Color}
}

// UpdateStageRequest is the body of PATCH /v1/pipelines/{id}/stages/{stageId}.
type UpdateStageRequest struct {
	Name   *string `json:"name"`
	Order  *int    `json:"order"`
	IsWon  *bool   `json:"is_won"`
	IsLost *bool   `json:"is_lost"`
	Color  *string `json:"color"`
}

// ToCommand maps to the service command.
func (r UpdateStageRequest) ToCommand() ccontracts.UpdateStage {
	return ccontracts.UpdateStage{Name: r.Name, Order: r.Order, IsWon: r.IsWon, IsLost: r.IsLost, Color: r.Color}
}

// ReorderStagesRequest is the body of POST /v1/pipelines/{id}/stages/reorder.
type ReorderStagesRequest struct {
	StageIDs []string `json:"stage_ids"`
}

// ToCommand maps to the service command.
func (r ReorderStagesRequest) ToCommand() ccontracts.ReorderStages {
	return ccontracts.ReorderStages{StageIDs: r.StageIDs}
}

// ── responses ────────────────────────────────────────────────────────────────

// StageResponse is the public representation of a stage.
type StageResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Order  int    `json:"order"`
	IsWon  bool   `json:"is_won"`
	IsLost bool   `json:"is_lost"`
	Color  string `json:"color,omitempty"`
}

// PipelineResponse is the public representation of a pipeline. Stages are ordered
// by Order.
type PipelineResponse struct {
	ID        string          `json:"id"`
	TenantID  string          `json:"tenant_id"`
	Name      string          `json:"name"`
	IsDefault bool            `json:"is_default"`
	Stages    []StageResponse `json:"stages"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// NewPipelineResponse maps a pipeline entity to its DTO (stages sorted by Order).
func NewPipelineResponse(p *entity.Pipeline) PipelineResponse {
	p.SortStages()
	stages := make([]StageResponse, len(p.Stages))
	for i, st := range p.Stages {
		stages[i] = StageResponse{ID: st.ID, Name: st.Name, Order: st.Order, IsWon: st.IsWon, IsLost: st.IsLost, Color: st.Color}
	}
	return PipelineResponse{
		ID: p.ID, TenantID: p.TenantID, Name: p.Name, IsDefault: p.IsDefault,
		Stages: stages, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

// NewPipelineResponses maps a slice.
func NewPipelineResponses(items []*entity.Pipeline) []PipelineResponse {
	out := make([]PipelineResponse, len(items))
	for i, p := range items {
		out[i] = NewPipelineResponse(p)
	}
	return out
}

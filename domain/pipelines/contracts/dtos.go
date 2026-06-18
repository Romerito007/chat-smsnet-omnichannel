// Package contracts holds the pipelines service inputs (commands) and ports.
package contracts

import "context"

// StageInput is one stage in a CreatePipeline command.
type StageInput struct {
	Name   string
	Order  int
	IsWon  bool
	IsLost bool
	Color  string
}

// CreatePipeline creates a funnel with its initial stages.
type CreatePipeline struct {
	Name   string
	Stages []StageInput
}

// UpdatePipeline renames the pipeline and/or sets it as the tenant default. Nil
// fields are left unchanged.
type UpdatePipeline struct {
	Name      *string
	IsDefault *bool
}

// AddStage appends a stage to a pipeline.
type AddStage struct {
	Name   string
	Order  int
	IsWon  bool
	IsLost bool
	Color  string
}

// UpdateStage edits a stage in place. Nil fields are left unchanged.
type UpdateStage struct {
	Name   *string
	Order  *int
	IsWon  *bool
	IsLost *bool
	Color  *string
}

// ReorderStages assigns each stage its new Order from the position in StageIDs.
type ReorderStages struct {
	StageIDs []string
}

// StageDealChecker reports whether a stage still holds deals, so the service can
// refuse to delete a non-empty stage. Implemented by the deals domain in a later
// block; optional until then (a nil checker means "no deals yet").
type StageDealChecker interface {
	StageHasDeals(ctx context.Context, pipelineID, stageID string) (bool, error)
}

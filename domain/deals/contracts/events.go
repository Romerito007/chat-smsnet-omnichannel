package contracts

// Realtime event names emitted by the deals service so the Kanban reacts live
// (same WebSocket fan-out as conversation/notification events).
const (
	// RealtimeDealStageChanged fires whenever a deal moves stage — manual drag,
	// automation, or mark-lost (any origin).
	RealtimeDealStageChanged = "deal.stage_changed"
	// RealtimeDealCreated fires when a new opportunity card is created.
	RealtimeDealCreated = "deal.created"
	// RealtimeDealUpdated fires on an edit (title/value/seller/sector/…) so the card
	// refreshes without a reload.
	RealtimeDealUpdated = "deal.updated"
)

// DealEvent is the realtime payload for a deal Kanban event. FromStageID is set only
// on a stage change (empty for created/updated); MovedBy ("user"|"automation") tags a
// stage change's origin. The client keys/dedupes on DealID.
type DealEvent struct {
	DealID      string `json:"deal_id"`
	PipelineID  string `json:"pipeline_id"`
	FromStageID string `json:"from_stage_id,omitempty"`
	ToStageID   string `json:"to_stage_id"`
	Status      string `json:"status"`
	MovedBy     string `json:"moved_by,omitempty"`
	AssignedTo  string `json:"assigned_to,omitempty"`
}

// Package entity holds the deal-timeline aggregate: a chronological feed of a deal's
// business events (automatic ones written by the deals service, plus manual seller
// comments). Future blocks (tasks, products) append their own events here.
package entity

import "time"

// Kind enumerates the timeline event types. Task/product kinds are reserved for the
// later blocks (the enum is prepared; the deals service emits only the rest).
type Kind string

const (
	KindDealCreated     Kind = "deal_created"
	KindStageChanged    Kind = "stage_changed"
	KindValueChanged    Kind = "value_changed"
	KindAssignedChanged Kind = "assigned_changed"
	KindPriorityChanged Kind = "priority_changed"
	KindComment         Kind = "comment"
	KindTaskCreated     Kind = "task_created"
	KindTaskCompleted   Kind = "task_completed"
	KindProductAdded    Kind = "product_added"
	KindProductRemoved  Kind = "product_removed"
	KindWon             Kind = "won"
	KindLost            Kind = "lost"
)

// AllKinds is the closed catalog of timeline kinds.
var AllKinds = []Kind{
	KindDealCreated, KindStageChanged, KindValueChanged, KindAssignedChanged,
	KindPriorityChanged, KindComment, KindTaskCreated, KindTaskCompleted,
	KindProductAdded, KindProductRemoved, KindWon, KindLost,
}

// ValidKind reports whether k is a known kind.
func ValidKind(k Kind) bool {
	for _, x := range AllKinds {
		if x == k {
			return true
		}
	}
	return false
}

// Non-human actor ids, used when no user caused the event.
const (
	ActorSystem     = "system"
	ActorAutomation = "automation"
)

// Event is one entry on a deal's timeline. Data carries the kind-specific fields
// (e.g. stage_changed → {from_stage_id, to_stage_id}; comment → {text};
// value_changed → {from, to}); names are resolved at read time, never stored raw.
type Event struct {
	ID        string
	TenantID  string
	DealID    string
	Kind      Kind
	ActorID   string // the user who caused it, or "system"/"automation"
	Data      map[string]any
	CreatedAt time.Time
}

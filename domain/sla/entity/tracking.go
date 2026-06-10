package entity

import "time"

// TrackingStatus is the lifecycle of an SLA tracking record. It stays "running"
// while the conversation is open (even after a target is breached — the breach
// flags record that); it is finalized to met/breached on resolution.
type TrackingStatus string

const (
	StatusRunning  TrackingStatus = "running"
	StatusMet      TrackingStatus = "met"
	StatusBreached TrackingStatus = "breached"
)

// SLATracking measures one conversation against its policy. The due and warn
// instants are absolute, pre-computed at creation (already accounting for
// business hours), so the scheduler only compares timestamps.
type SLATracking struct {
	ID             string
	TenantID       string
	ConversationID string
	PolicyID       string
	SectorID       string

	FirstResponseDueAt  *time.Time
	FirstResponseWarnAt *time.Time
	ResolutionDueAt     *time.Time
	ResolutionWarnAt    *time.Time

	FirstResponseAt *time.Time
	ResolvedAt      *time.Time

	FirstResponseBreached bool
	ResolutionBreached    bool
	FirstResponseWarned   bool
	ResolutionWarned      bool

	// PauseOnWaitingCustomer (denormalized from the policy) suppresses
	// warning/breach alerts while the conversation waits on the customer.
	PauseOnWaitingCustomer bool

	Status    TrackingStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Finalize sets the terminal status from the breach flags (called on resolve).
func (t *SLATracking) Finalize() {
	if t.FirstResponseBreached || t.ResolutionBreached {
		t.Status = StatusBreached
	} else {
		t.Status = StatusMet
	}
}

// AtRisk reports whether a running tracking has fired a warning or breach.
func (t *SLATracking) AtRisk() bool {
	return t.FirstResponseWarned || t.ResolutionWarned || t.FirstResponseBreached || t.ResolutionBreached
}

// Package contracts holds the SLA service inputs and event names.
package contracts

// Realtime/webhook event names emitted by the SLA domain.
const (
	RealtimeSLAWarning  = "sla.warning"
	RealtimeSLABreached = "sla.breached"
)

// CreatePolicy is the input to create an SLA policy.
type CreatePolicy struct {
	Name                   string
	SectorIDs              []string
	Priority               string
	Channel                string
	FirstResponseTargetSec int
	ResolutionTargetSec    int
	BusinessHoursOnly      bool
	WarningThresholdPct    int
	PauseOnWaitingCustomer bool
	Enabled                *bool
}

// UpdatePolicy patches a policy. Nil fields are left unchanged.
type UpdatePolicy struct {
	Name                   *string
	SectorIDs              *[]string
	Priority               *string
	Channel                *string
	FirstResponseTargetSec *int
	ResolutionTargetSec    *int
	BusinessHoursOnly      *bool
	WarningThresholdPct    *int
	PauseOnWaitingCustomer *bool
	Enabled                *bool
}

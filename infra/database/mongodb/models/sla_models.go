package models

import "time"

// SLAPolicy is the BSON document for an SLA policy.
type SLAPolicy struct {
	Base                   `bson:",inline"`
	Name                   string   `bson:"name"`
	SectorIDs              []string `bson:"sector_ids,omitempty"`
	Priority               string   `bson:"priority,omitempty"`
	Channel                string   `bson:"channel,omitempty"`
	FirstResponseTargetSec int      `bson:"first_response_target_seconds"`
	ResolutionTargetSec    int      `bson:"resolution_target_seconds"`
	BusinessHoursOnly      bool     `bson:"business_hours_only"`
	WarningThresholdPct    int      `bson:"warning_threshold_percent"`
	PauseOnWaitingCustomer bool     `bson:"pause_on_waiting_customer"`
	Enabled                bool     `bson:"enabled"`
}

// SLATracking is the BSON document for a per-conversation SLA tracking.
type SLATracking struct {
	ID                     string     `bson:"_id"`
	TenantID               string     `bson:"tenant_id"`
	ConversationID         string     `bson:"conversation_id"`
	PolicyID               string     `bson:"policy_id"`
	SectorID               string     `bson:"sector_id,omitempty"`
	FirstResponseDueAt     *time.Time `bson:"first_response_due_at,omitempty"`
	FirstResponseWarnAt    *time.Time `bson:"first_response_warn_at,omitempty"`
	ResolutionDueAt        *time.Time `bson:"resolution_due_at,omitempty"`
	ResolutionWarnAt       *time.Time `bson:"resolution_warn_at,omitempty"`
	FirstResponseAt        *time.Time `bson:"first_response_at,omitempty"`
	ResolvedAt             *time.Time `bson:"resolved_at,omitempty"`
	FirstResponseBreached  bool       `bson:"first_response_breached"`
	ResolutionBreached     bool       `bson:"resolution_breached"`
	FirstResponseWarned    bool       `bson:"first_response_warned"`
	ResolutionWarned       bool       `bson:"resolution_warned"`
	PauseOnWaitingCustomer bool       `bson:"pause_on_waiting_customer"`
	Status                 string     `bson:"status"`
	CreatedAt              time.Time  `bson:"created_at"`
	UpdatedAt              time.Time  `bson:"updated_at"`
}

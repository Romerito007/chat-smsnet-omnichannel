// Package sla holds the request/response DTOs for the SLA endpoints.
package sla

import (
	"time"

	slacontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/sla/contracts"
	slaentity "github.com/romerito007/chat-smsnet-omnichannel/domain/sla/entity"
)

// ── policies ─────────────────────────────────────────────────────────────────

type CreatePolicyRequest struct {
	Name                   string   `json:"name"`
	SectorIDs              []string `json:"sector_ids"`
	Priority               string   `json:"priority"`
	Channel                string   `json:"channel"`
	FirstResponseTargetSec int      `json:"first_response_target_seconds"`
	ResolutionTargetSec    int      `json:"resolution_target_seconds"`
	BusinessHoursOnly      bool     `json:"business_hours_only"`
	WarningThresholdPct    int      `json:"warning_threshold_percent"`
	PauseOnWaitingCustomer bool     `json:"pause_on_waiting_customer"`
	Enabled                *bool    `json:"enabled"`
}

func (r CreatePolicyRequest) ToCommand() slacontracts.CreatePolicy {
	return slacontracts.CreatePolicy{
		Name: r.Name, SectorIDs: r.SectorIDs, Priority: r.Priority, Channel: r.Channel,
		FirstResponseTargetSec: r.FirstResponseTargetSec, ResolutionTargetSec: r.ResolutionTargetSec,
		BusinessHoursOnly: r.BusinessHoursOnly, WarningThresholdPct: r.WarningThresholdPct,
		PauseOnWaitingCustomer: r.PauseOnWaitingCustomer, Enabled: r.Enabled,
	}
}

type UpdatePolicyRequest struct {
	Name                   *string   `json:"name"`
	SectorIDs              *[]string `json:"sector_ids"`
	Priority               *string   `json:"priority"`
	Channel                *string   `json:"channel"`
	FirstResponseTargetSec *int      `json:"first_response_target_seconds"`
	ResolutionTargetSec    *int      `json:"resolution_target_seconds"`
	BusinessHoursOnly      *bool     `json:"business_hours_only"`
	WarningThresholdPct    *int      `json:"warning_threshold_percent"`
	PauseOnWaitingCustomer *bool     `json:"pause_on_waiting_customer"`
	Enabled                *bool     `json:"enabled"`
}

func (r UpdatePolicyRequest) ToCommand() slacontracts.UpdatePolicy {
	return slacontracts.UpdatePolicy{
		Name: r.Name, SectorIDs: r.SectorIDs, Priority: r.Priority, Channel: r.Channel,
		FirstResponseTargetSec: r.FirstResponseTargetSec, ResolutionTargetSec: r.ResolutionTargetSec,
		BusinessHoursOnly: r.BusinessHoursOnly, WarningThresholdPct: r.WarningThresholdPct,
		PauseOnWaitingCustomer: r.PauseOnWaitingCustomer, Enabled: r.Enabled,
	}
}

type PolicyResponse struct {
	ID                     string    `json:"id"`
	TenantID               string    `json:"tenant_id"`
	Name                   string    `json:"name"`
	SectorIDs              []string  `json:"sector_ids,omitempty"`
	Priority               string    `json:"priority,omitempty"`
	Channel                string    `json:"channel,omitempty"`
	FirstResponseTargetSec int       `json:"first_response_target_seconds"`
	ResolutionTargetSec    int       `json:"resolution_target_seconds"`
	BusinessHoursOnly      bool      `json:"business_hours_only"`
	WarningThresholdPct    int       `json:"warning_threshold_percent"`
	PauseOnWaitingCustomer bool      `json:"pause_on_waiting_customer"`
	Enabled                bool      `json:"enabled"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

func NewPolicyResponse(p *slaentity.SLAPolicy) PolicyResponse {
	return PolicyResponse{
		ID: p.ID, TenantID: p.TenantID, Name: p.Name, SectorIDs: p.SectorIDs,
		Priority: p.Priority, Channel: p.Channel,
		FirstResponseTargetSec: p.FirstResponseTargetSec, ResolutionTargetSec: p.ResolutionTargetSec,
		BusinessHoursOnly: p.BusinessHoursOnly, WarningThresholdPct: p.WarningThresholdPct,
		PauseOnWaitingCustomer: p.PauseOnWaitingCustomer, Enabled: p.Enabled,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func NewPolicyResponses(items []*slaentity.SLAPolicy) []PolicyResponse {
	out := make([]PolicyResponse, 0, len(items))
	for _, p := range items {
		out = append(out, NewPolicyResponse(p))
	}
	return out
}

// ── tracking ─────────────────────────────────────────────────────────────────

type TrackingResponse struct {
	ID                    string     `json:"id"`
	ConversationID        string     `json:"conversation_id"`
	PolicyID              string     `json:"policy_id"`
	Status                string     `json:"status"`
	FirstResponseDueAt    *time.Time `json:"first_response_due_at,omitempty"`
	ResolutionDueAt       *time.Time `json:"resolution_due_at,omitempty"`
	FirstResponseAt       *time.Time `json:"first_response_at,omitempty"`
	ResolvedAt            *time.Time `json:"resolved_at,omitempty"`
	FirstResponseBreached bool       `json:"first_response_breached"`
	ResolutionBreached    bool       `json:"resolution_breached"`
	FirstResponseWarned   bool       `json:"first_response_warned"`
	ResolutionWarned      bool       `json:"resolution_warned"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

func NewTrackingResponse(t *slaentity.SLATracking) TrackingResponse {
	return TrackingResponse{
		ID: t.ID, ConversationID: t.ConversationID, PolicyID: t.PolicyID, Status: string(t.Status),
		FirstResponseDueAt: t.FirstResponseDueAt, ResolutionDueAt: t.ResolutionDueAt,
		FirstResponseAt: t.FirstResponseAt, ResolvedAt: t.ResolvedAt,
		FirstResponseBreached: t.FirstResponseBreached, ResolutionBreached: t.ResolutionBreached,
		FirstResponseWarned: t.FirstResponseWarned, ResolutionWarned: t.ResolutionWarned,
		CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
}

func NewTrackingResponses(items []*slaentity.SLATracking) []TrackingResponse {
	out := make([]TrackingResponse, 0, len(items))
	for _, t := range items {
		out = append(out, NewTrackingResponse(t))
	}
	return out
}

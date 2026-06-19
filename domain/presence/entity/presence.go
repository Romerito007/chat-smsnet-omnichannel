// Package entity holds the AgentPresence aggregate. Presence is operational
// state kept in Redis, not MongoDB.
package entity

import "time"

// Status is an agent's operational availability state.
type Status string

const (
	StatusOffline   Status = "offline"
	StatusOnline    Status = "online"
	StatusAvailable Status = "available"
	StatusBusy      Status = "busy"
	StatusAway      Status = "away"
	StatusPaused    Status = "paused"
	StatusLunch     Status = "lunch"
	StatusTraining  Status = "training"
)

// Valid reports whether s is a known status.
func (s Status) Valid() bool {
	switch s {
	case StatusOffline, StatusOnline, StatusAvailable, StatusBusy,
		StatusAway, StatusPaused, StatusLunch, StatusTraining:
		return true
	}
	return false
}

// IsConnected reports whether the status implies an active (non-offline) session.
func (s Status) IsConnected() bool {
	return s != StatusOffline && s != ""
}

// CanReceive reports whether an agent in this EFFECTIVE status is eligible to receive
// auto-routed conversations (online — or the legacy "available").
func (s Status) CanReceive() bool {
	return s == StatusOnline || s == StatusAvailable
}

// Availability is the agent's DURABLE manual choice: online, away or offline.
type Availability string

const (
	AvailabilityOnline  Availability = "online"
	AvailabilityAway    Availability = "away"
	AvailabilityOffline Availability = "offline"
)

// ResolveEffective applies the presence precedence to compute the EFFECTIVE status:
//   - availability offline           → offline (always; a live socket does not change it)
//   - availability away (or any other
//     non-online "away-like" value)  → that same status (sticky until changed manually)
//   - availability online:
//     has a live socket            → online
//     no live socket               → autoOffline ? offline : online
func ResolveEffective(availability string, autoOffline, hasLiveSocket bool) Status {
	switch availability {
	case string(AvailabilityOffline):
		return StatusOffline
	case string(AvailabilityOnline), "":
		if hasLiveSocket {
			return StatusOnline
		}
		if autoOffline {
			return StatusOffline
		}
		return StatusOnline
	default:
		// away / busy / paused / lunch / training → sticky, socket-independent.
		return Status(availability)
	}
}

// AgentPresence is the live state of an agent. CurrentLoad is derived from open
// conversations assigned to the agent and is recomputed on writes/reads.
type AgentPresence struct {
	TenantID string
	UserID   string
	// Status is the EFFECTIVE status (precedence applied).
	Status Status
	// Availability is the raw DURABLE manual choice (online|away|offline); AutoOffline
	// is the per-agent toggle. Both come from the user document and let the front
	// render the two controls.
	Availability       string
	AutoOffline        bool
	CurrentLoad        int
	MaxConcurrentChats int
	LastSeenAt         time.Time
}

// HasCapacity reports whether the agent can take another conversation.
func (p *AgentPresence) HasCapacity() bool {
	return p.MaxConcurrentChats <= 0 || p.CurrentLoad < p.MaxConcurrentChats
}

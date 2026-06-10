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

// AgentPresence is the live state of an agent. CurrentLoad is derived from open
// conversations assigned to the agent and is recomputed on writes/reads.
type AgentPresence struct {
	TenantID           string
	UserID             string
	Status             Status
	CurrentLoad        int
	MaxConcurrentChats int
	LastSeenAt         time.Time
}

// HasCapacity reports whether the agent can take another conversation.
func (p *AgentPresence) HasCapacity() bool {
	return p.MaxConcurrentChats <= 0 || p.CurrentLoad < p.MaxConcurrentChats
}

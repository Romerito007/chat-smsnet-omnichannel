// Package entity holds the conversations domain aggregates: Conversation,
// Message and ConversationEvent.
package entity

import "time"

// Status is the conversation lifecycle state.
type Status string

const (
	StatusNew             Status = "new"
	StatusAutomation      Status = "automation"
	StatusQueued          Status = "queued"
	StatusAssigned        Status = "assigned"
	StatusWaitingCustomer Status = "waiting_customer"
	StatusWaitingAgent    Status = "waiting_agent"
	StatusTransferred     Status = "transferred"
	StatusResolved        Status = "resolved"
	StatusClosed          Status = "closed"
	StatusArchived        Status = "archived"
)

// Valid reports whether s is a known status.
func (s Status) Valid() bool {
	switch s {
	case StatusNew, StatusAutomation, StatusQueued, StatusAssigned,
		StatusWaitingCustomer, StatusWaitingAgent, StatusTransferred,
		StatusResolved, StatusClosed, StatusArchived:
		return true
	}
	return false
}

// IsClosed reports whether the conversation is in a terminal state.
func (s Status) IsClosed() bool {
	return s == StatusResolved || s == StatusClosed || s == StatusArchived
}

// Priority ranks a conversation for routing/attention.
type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityNormal Priority = "normal"
	PriorityHigh   Priority = "high"
	PriorityUrgent Priority = "urgent"
)

// Valid reports whether p is a known priority.
func (p Priority) Valid() bool {
	switch p {
	case PriorityLow, PriorityNormal, PriorityHigh, PriorityUrgent:
		return true
	}
	return false
}

// Conversation is a thread of messages between a contact and the operation.
type Conversation struct {
	ID            string
	TenantID      string
	ContactID     string
	Channel       string
	SectorID      string
	QueueID       string
	Status        Status
	AssignedTo    string
	Priority      Priority
	Tags          []string
	LastMessageAt time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
	ClosedAt      *time.Time
}

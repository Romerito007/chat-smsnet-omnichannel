// Package entity holds the Deal aggregate: a sales opportunity (the Kanban card)
// moving across a pipeline's stages.
package entity

import "time"

// Status is the deal lifecycle. A deal is open while in a non-terminal stage, and
// becomes won/lost when it enters an IsWon/IsLost stage.
type Status string

const (
	StatusOpen Status = "open"
	StatusWon  Status = "won"
	StatusLost Status = "lost"
)

// DefaultCurrency is used when a deal is created without one.
const DefaultCurrency = "BRL"

// Deal is a sales opportunity. It belongs to a pipeline and sits in one of its
// stages; ConversationIDs links the chat conversations it came from / relates to.
type Deal struct {
	ID         string
	TenantID   string
	PipelineID string
	StageID    string
	ContactID  string
	Title      string
	Value      float64
	Currency   string
	// AssignedTo is the seller (agent) handling the deal; optional.
	AssignedTo string
	// SectorID scopes visibility (mirrors conversations); optional.
	SectorID          string
	ConversationIDs   []string
	Source            string
	Status            Status
	LostReason        string
	ExpectedCloseDate *time.Time
	// StageChangedAt is bumped on every stage move (measures time-in-stage later).
	StageChangedAt time.Time
	ClosedAt       *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// HasConversation reports whether the deal already links the given conversation.
func (d *Deal) HasConversation(conversationID string) bool {
	for _, id := range d.ConversationIDs {
		if id == conversationID {
			return true
		}
	}
	return false
}

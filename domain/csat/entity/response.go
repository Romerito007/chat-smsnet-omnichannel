package entity

import "time"

// Status is the lifecycle of a CSAT response.
type Status string

const (
	StatusSent      Status = "sent"      // created/sent, awaiting an answer
	StatusResponded Status = "responded" // the customer answered
	StatusExpired   Status = "expired"   // no answer within the window
)

// CSATResponse tracks one survey sent for one conversation. Token is the public,
// unguessable handle the customer uses to answer without exposing the
// conversation.
type CSATResponse struct {
	ID             string
	TenantID       string
	ConversationID string
	ContactID      string
	SurveyID       string
	AgentID        string
	Token          string
	Score          *int
	Comment        string
	SentAt         *time.Time
	RespondedAt    *time.Time
	Status         Status
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Answerable reports whether the response can still be answered.
func (r *CSATResponse) Answerable() bool { return r.Status == StatusSent }

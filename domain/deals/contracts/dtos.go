// Package contracts holds the deals service inputs (commands/queries) and the ports
// it depends on (pipeline lookup, conversation lookup, contact check).
package contracts

import (
	"context"
	"time"

	pipelineentity "github.com/romerito007/chat-smsnet-omnichannel/domain/pipelines/entity"
)

// CreateDeal creates an opportunity. PipelineID/StageID default to the tenant's
// default pipeline and its first stage when empty.
type CreateDeal struct {
	Title             string
	Value             float64
	Currency          string
	PipelineID        string
	StageID           string
	ContactID         string
	AssignedTo        string
	SectorID          string
	Source            string
	ExpectedCloseDate *time.Time
}

// UpdateDeal edits the editable fields. Nil = unchanged.
type UpdateDeal struct {
	Title             *string
	Value             *float64
	Currency          *string
	AssignedTo        *string
	SectorID          *string
	Source            *string
	ExpectedCloseDate *time.Time
	ClearExpectedDate bool
}

// CreateFromConversation creates a deal pre-linked to a conversation and its contact.
type CreateFromConversation struct {
	ConversationID string
	Title          string
	Value          float64
	Currency       string
}

// ListFilter narrows a deal listing. Empty fields are ignored. Tenant scope and
// visibility are applied separately by the service.
type ListFilter struct {
	PipelineID string
	StageID    string
	AssignedTo string
	ContactID  string
	Status     string
	Q          string
}

// Visibility constrains which deals an actor may see. When All is true the actor
// sees every deal in the tenant; otherwise only those assigned to UserID or in one
// of SectorIDs.
type Visibility struct {
	All       bool
	SectorIDs []string
	UserID    string
}

// PipelineLookup resolves pipelines (with their stages) for the deal flow: validate
// the stage belongs to the pipeline, read its terminal flags/name, and find the
// tenant default + first stage. Implemented by the pipelines service.
type PipelineLookup interface {
	Get(ctx context.Context, pipelineID string) (*pipelineentity.Pipeline, error)
	Default(ctx context.Context) (*pipelineentity.Pipeline, error)
}

// ConversationRef is the minimal conversation data the deal flow needs.
type ConversationRef struct {
	ContactID  string
	SectorID   string
	AssignedTo string
}

// ConversationLookup resolves a conversation's contact/sector. Implemented over the
// conversations repository.
type ConversationLookup interface {
	Conversation(ctx context.Context, conversationID string) (*ConversationRef, error)
}

// ContactChecker reports whether a contact exists in the tenant. Implemented over
// the contacts service.
type ContactChecker interface {
	ContactExists(ctx context.Context, contactID string) (bool, error)
}

// DealAudience resolves the user ids of a sector's agents, so an automated deal move
// with no owner still reaches the team that can see the sector. Implemented over the
// IAM user service (ListBySector). Optional.
type DealAudience interface {
	SectorAgents(ctx context.Context, sectorID string) ([]string, error)
}

// Timeline event kinds the deals service writes (the others — comment, task_*,
// product_* — are written by the timeline/tasks/products blocks).
const (
	TimelineDealCreated     = "deal_created"
	TimelineStageChanged    = "stage_changed"
	TimelineValueChanged    = "value_changed"
	TimelineAssignedChanged = "assigned_changed"
	TimelineWon             = "won"
	TimelineLost            = "lost"
)

// TimelineEvent is one automatic event the deals service records on a deal's
// timeline. Data carries the kind-specific fields (ids only; names resolved at read).
type TimelineEvent struct {
	DealID  string
	Kind    string
	ActorID string
	Data    map[string]any
}

// TimelineWriter appends an automatic event to a deal's timeline. Implemented by the
// dealtimeline service (via a factory adapter). Best-effort and fire-and-forget — a
// timeline failure must never break the deal action that produced it. Optional.
type TimelineWriter interface {
	Record(ctx context.Context, ev TimelineEvent)
}

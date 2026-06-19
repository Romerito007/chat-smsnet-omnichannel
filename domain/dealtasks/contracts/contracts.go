// Package contracts holds the deal-task service inputs, the ports it depends on (deal
// lookup for visibility, the tasks toggle, the agent checker/directory, the timeline
// writer) and the enriched task view.
package contracts

import (
	"context"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// CreateTask is the input to create a deal task.
type CreateTask struct {
	DealID      string
	Title       string
	Description string
	DueDate     *time.Time
	AssignedTo  string
}

// UpdateTask edits the editable fields. Nil = unchanged; ClearDueDate/ClearAssignee
// remove the optional fields.
type UpdateTask struct {
	Title        *string
	Description  *string
	DueDate      *time.Time
	ClearDueDate bool
	AssignedTo   *string
}

// ListFilter narrows the consolidated task listing (GET /v1/crm/tasks).
type ListFilter struct {
	AssignedTo string
	Status     string
	DueBefore  *time.Time
}

// DealRef is the minimal deal data the tasks need for the visibility check (same rule
// as the deal list).
type DealRef struct {
	TenantID   string
	SectorID   string
	AssignedTo string
}

// DealLookup resolves a deal (tenant-scoped) for the visibility check. Implemented
// over the deals repository.
type DealLookup interface {
	Deal(ctx context.Context, dealID string) (*DealRef, error)
}

// ModuleGate reports whether the tasks module is enabled for the tenant. Implemented
// over the crmsettings service. Optional: a nil gate means always enabled.
type ModuleGate interface {
	TasksEnabled(ctx context.Context) (bool, error)
}

// AgentChecker reports whether a user id is an agent of the tenant (to validate an
// assignee). Implemented over the IAM user service. Optional.
type AgentChecker interface {
	AgentExists(ctx context.Context, userID string) (bool, error)
}

// AgentDirectory resolves user ids to display cards (name + avatar) so a task shows
// the assignee/creator, not a raw id. Optional.
type AgentDirectory interface {
	AgentCards(ctx context.Context, userIDs []string) (map[string]shared.DisplayCard, error)
}

// TimelineEvent is one event the tasks service records on the deal's timeline.
type TimelineEvent struct {
	DealID  string
	Kind    string
	ActorID string
	Data    map[string]any
}

// TimelineWriter appends a task event (task_created/task_completed) to the deal's
// timeline. Implemented by the dealtimeline service (factory adapter). Best-effort.
type TimelineWriter interface {
	Record(ctx context.Context, ev TimelineEvent)
}

// TaskView is the enriched task response: the task plus the resolved assignee/creator
// names (never a raw id).
type TaskView struct {
	ID                  string     `json:"id"`
	DealID              string     `json:"deal_id"`
	Title               string     `json:"title"`
	Description         string     `json:"description,omitempty"`
	DueDate             *time.Time `json:"due_date,omitempty"`
	AssignedTo          string     `json:"assigned_to,omitempty"`
	AssignedToName      string     `json:"assigned_to_name,omitempty"`
	AssignedToAvatarURL string     `json:"assigned_to_avatar_url,omitempty"`
	Status              string     `json:"status"`
	CompletedAt         *time.Time `json:"completed_at,omitempty"`
	CreatedBy           string     `json:"created_by,omitempty"`
	CreatedByName       string     `json:"created_by_name,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// Package entity holds the deal-task aggregate: a seller follow-up attached to a deal
// (call, send proposal, schedule a visit) with an optional due date and assignee, and
// a pending|done status.
package entity

import "time"

// Status is a task's lifecycle.
type Status string

const (
	StatusPending Status = "pending"
	StatusDone    Status = "done"
)

// ValidStatus reports whether s is a known status.
func ValidStatus(s Status) bool { return s == StatusPending || s == StatusDone }

// DealTask is a follow-up on a deal.
type DealTask struct {
	ID          string
	TenantID    string
	DealID      string
	Title       string
	Description string
	DueDate     *time.Time
	AssignedTo  string
	Status      Status
	CompletedAt *time.Time
	CreatedBy   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// IsDone reports whether the task is completed.
func (t *DealTask) IsDone() bool { return t.Status == StatusDone }

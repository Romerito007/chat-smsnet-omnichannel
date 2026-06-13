package entity

import "time"

// EvalStatus is the outcome of evaluating one rule for one event occurrence.
type EvalStatus string

const (
	// EvalActionEnqueued: the rule matched and its action(s) were enqueued.
	EvalActionEnqueued EvalStatus = "action_enqueued"
	// EvalSkippedDedup: the rule matched but was skipped by the anti-loop dedup
	// window (same rule+conversation+event fired recently).
	EvalSkippedDedup EvalStatus = "skipped_dedup"
	// EvalError: an action failed to enqueue (e.g. webhook disabled/gone).
	EvalError EvalStatus = "error"
)

// RuleEvaluationLog is the minimal record of a rule firing. It stores no event
// payload — only metadata for diagnostics.
type RuleEvaluationLog struct {
	ID             string
	TenantID       string
	RuleID         string
	Event          RuleEvent
	ConversationID string
	Status         EvalStatus
	ErrorSummary   string
	CreatedAt      time.Time
}

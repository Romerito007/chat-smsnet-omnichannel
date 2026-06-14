package entity

import "time"

// EvalStatus is the outcome of evaluating one rule for one event occurrence.
type EvalStatus string

const (
	// EvalActionEnqueued: the rule matched and its action(s) were enqueued.
	EvalActionEnqueued EvalStatus = "action_enqueued"
	// EvalSkippedDedup: the rule matched but was skipped by the anti-loop dedup —
	// the (rule, event_id) firing was already claimed (e.g. a task retry).
	EvalSkippedDedup EvalStatus = "skipped_dedup"
	// EvalSkippedAutomation: the event was produced by an automation action
	// (origin=automation) and is suppressed so automation never feeds itself.
	EvalSkippedAutomation EvalStatus = "skipped_automation"
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

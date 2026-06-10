package entity

import "time"

// RunStatus is the lifecycle of one automation execution.
type RunStatus string

const (
	RunStarted         RunStatus = "started"
	RunWaitingCallback RunStatus = "waiting_callback"
	RunCompleted       RunStatus = "completed"
	RunFailed          RunStatus = "failed"
	RunTimeout         RunStatus = "timeout"
)

// IsTerminal reports whether the run has reached a final state.
func (s RunStatus) IsTerminal() bool {
	return s == RunCompleted || s == RunFailed || s == RunTimeout
}

// AutomationRun records a single invocation of the external flow for a
// conversation, including its input, the flow's output and the outcome.
type AutomationRun struct {
	ID             string
	TenantID       string
	ConversationID string
	MessageID      string
	ExternalRunID  string
	Status         RunStatus
	Input          map[string]any
	Output         map[string]any
	Error          string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

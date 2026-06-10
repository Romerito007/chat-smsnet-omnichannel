package entity

import "time"

// Action names a copilot operation.
type Action string

const (
	ActionSuggestReply Action = "suggest_reply"
	ActionSummarize    Action = "summarize"
	ActionClassify     Action = "classify"
	ActionNextAction   Action = "next_action"
)

// LogStatus is the outcome of a copilot call.
type LogStatus string

const (
	StatusSuccess         LogStatus = "success"
	StatusError           LogStatus = "error"
	StatusPendingApproval LogStatus = "pending_approval"
	StatusBlocked         LogStatus = "blocked"
)

// AILog is the per-call audit record. It stores only summaries of the input and
// output (never the full prompt or raw data), plus token counts and cost for
// billing/observability.
type AILog struct {
	ID             string
	TenantID       string
	UserID         string
	ConversationID string
	Provider       string
	Model          string
	Action         Action
	InputSummary   string
	OutputSummary  string
	TokensInput    int
	TokensOutput   int
	EstimatedCost  float64
	Status         LogStatus
	Error          string
	CreatedAt      time.Time
}

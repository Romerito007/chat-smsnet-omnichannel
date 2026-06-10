// Package contracts holds the CSAT service inputs, the Asynq task payloads and
// the channel-sender / enqueuer ports.
package contracts

import "context"

// CreateSurvey is the input to create a survey.
type CreateSurvey struct {
	Name         string
	Scale        string
	QuestionText string
	SectorIDs    []string
	DelaySeconds int
	Enabled      *bool
}

// UpdateSurvey patches a survey. Nil fields are left unchanged.
type UpdateSurvey struct {
	Name         *string
	Scale        *string
	QuestionText *string
	SectorIDs    *[]string
	DelaySeconds *int
	Enabled      *bool
}

// SubmitResponse is the public answer payload.
type SubmitResponse struct {
	Score   int
	Comment string
}

// SendTask is the csat.send Asynq payload.
type SendTask struct {
	TenantID   string `json:"tenant_id"`
	ResponseID string `json:"response_id"`
}

// ExpireTask is the csat.expire Asynq payload.
type ExpireTask struct {
	TenantID   string `json:"tenant_id"`
	ResponseID string `json:"response_id"`
}

// Enqueuer schedules the csat.send / csat.expire jobs.
type Enqueuer interface {
	EnqueueSend(task SendTask, delaySeconds int) error
	EnqueueExpire(task ExpireTask, delaySeconds int) error
}

// ChannelSender delivers the survey question to a conversation's channel. It is
// implemented by an adapter over the conversations/channels domains so CSAT
// reuses the existing outbound delivery (Prompt 9).
type ChannelSender interface {
	SendToConversation(ctx context.Context, conversationID, text string) error
}

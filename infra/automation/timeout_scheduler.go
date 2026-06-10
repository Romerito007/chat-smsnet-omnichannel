package automation

import (
	"encoding/json"
	"time"

	goasynq "github.com/hibiken/asynq"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/automation/contracts"
	infraasynq "github.com/romerito007/chat-smsnet-omnichannel/infra/asynq"
)

// TimeoutScheduler schedules the delayed automation.timeout job.
type TimeoutScheduler struct {
	client *infraasynq.Client
}

// NewTimeoutScheduler builds the scheduler.
func NewTimeoutScheduler(client *infraasynq.Client) *TimeoutScheduler {
	return &TimeoutScheduler{client: client}
}

// ScheduleTimeout enqueues automation.timeout to fire after delayMs.
func (s *TimeoutScheduler) ScheduleTimeout(task contracts.TimeoutTask, delayMs int) error {
	body, err := json.Marshal(task)
	if err != nil {
		return err
	}
	t := goasynq.NewTask(infraasynq.TaskAutomationTimeout, body)
	_, err = s.client.Enqueue(t,
		goasynq.Queue(infraasynq.QueueDefault),
		goasynq.ProcessIn(time.Duration(delayMs)*time.Millisecond),
		goasynq.MaxRetry(3),
	)
	return err
}

var _ contracts.TimeoutScheduler = (*TimeoutScheduler)(nil)

package start_routines

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	infraasynq "github.com/romerito007/chat-smsnet-omnichannel/infra/asynq"
)

// periodicJob declares a cron-scheduled task enqueued onto a queue.
type periodicJob struct {
	cronspec string
	taskType string
	queue    string
}

// scheduledJobs are the periodic, multi-tenant, idempotent jobs from section 4.
// Each handler (registered in bootstrap_workers) is responsible for fanning the
// work out across tenants when it runs.
var scheduledJobs = []periodicJob{
	{cronspec: "*/5 * * * *", taskType: infraasynq.TaskChatCloseInactive, queue: infraasynq.QueueDefault},
	{cronspec: "* * * * *", taskType: infraasynq.TaskSLACheck, queue: infraasynq.QueueCritical},
	{cronspec: "0 * * * *", taskType: infraasynq.TaskReportsSnapshot, queue: infraasynq.QueueReports},
	{cronspec: "30 3 * * *", taskType: infraasynq.TaskAuditCompact, queue: infraasynq.QueueReports},
	{cronspec: "15 4 * * *", taskType: infraasynq.TaskNotificationCleanup, queue: infraasynq.QueueReports},
	{cronspec: "*/5 * * * *", taskType: infraasynq.TaskChannelsHealth, queue: infraasynq.QueueDefault},
	{cronspec: "45 4 * * *", taskType: infraasynq.TaskPrivacyRetention, queue: infraasynq.QueueReports},
}

// runScheduler registers the periodic jobs and runs the Asynq scheduler until
// the context is cancelled.
func runScheduler(ctx context.Context, c *container.Container) error {
	scheduler := newAsynqScheduler(c)

	for _, job := range scheduledJobs {
		task := asynq.NewTask(job.taskType, nil)
		if _, err := scheduler.Register(job.cronspec, task, asynq.Queue(job.queue)); err != nil {
			return fmt.Errorf("register scheduled job %s: %w", job.taskType, err)
		}
	}

	if err := scheduler.Start(); err != nil {
		return fmt.Errorf("start scheduler: %w", err)
	}
	c.Logger.Info("asynq scheduler started", "jobs", len(scheduledJobs))

	<-ctx.Done()
	scheduler.Shutdown()
	return nil
}

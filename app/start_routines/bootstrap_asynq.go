package start_routines

import (
	"github.com/hibiken/asynq"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	infraasynq "github.com/romerito007/chat-smsnet-omnichannel/infra/asynq"
)

// newAsynqServer builds the worker server from the container config.
func newAsynqServer(c *container.Container) *asynq.Server {
	return infraasynq.NewServer(c.Config, c.Logger)
}

// newAsynqScheduler builds the periodic-task scheduler.
func newAsynqScheduler(c *container.Container) *asynq.Scheduler {
	return asynq.NewScheduler(infraasynq.RedisOpt(c.Config.Redis), &asynq.SchedulerOpts{
		LogLevel: asynq.InfoLevel,
	})
}

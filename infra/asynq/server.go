package asynq

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/romerito007/chat-smsnet-omnichannel/app/config"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// NewServer builds the worker server with the configured concurrency and queue
// priorities. Handlers are registered on the returned *asynq.ServeMux by the
// bootstrap_workers routine before the server is started.
func NewServer(cfg config.Config, logger shared.Logger) *asynq.Server {
	return asynq.NewServer(
		RedisOpt(cfg.Redis),
		asynq.Config{
			Concurrency: cfg.Asynq.Concurrency,
			Queues:      cfg.Asynq.Queues,
			ErrorHandler: asynq.ErrorHandlerFunc(func(_ context.Context, task *asynq.Task, err error) {
				logger.Error("asynq task failed",
					"type", task.Type(),
					"error", err.Error(),
				)
			}),
		},
	)
}

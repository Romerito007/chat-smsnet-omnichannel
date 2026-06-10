package asynq

import (
	"context"
	"time"

	"github.com/hibiken/asynq"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// LoggingMiddleware logs each job's type, duration and outcome. It is applied to
// the ServeMux in bootstrap_workers so every handler is observable.
func LoggingMiddleware(logger shared.Logger) asynq.MiddlewareFunc {
	return func(next asynq.Handler) asynq.Handler {
		return asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error {
			start := time.Now()
			err := next.ProcessTask(ctx, task)
			fields := []any{
				"type", task.Type(),
				"duration_ms", time.Since(start).Milliseconds(),
			}
			if err != nil {
				logger.Error("job processed", append(fields, "error", err.Error())...)
			} else {
				logger.Info("job processed", fields...)
			}
			return err
		})
	}
}

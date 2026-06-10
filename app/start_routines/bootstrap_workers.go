package start_routines

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	infraasynq "github.com/romerito007/chat-smsnet-omnichannel/infra/asynq"
)

// runWorker boots the Asynq worker: it builds the handler mux, registers every
// domain's job handlers, and serves until the context is cancelled.
func runWorker(ctx context.Context, c *container.Container) error {
	srv := newAsynqServer(c)
	mux := asynq.NewServeMux()
	mux.Use(infraasynq.LoggingMiddleware(c.Logger))

	registerHandlers(mux, c)

	if err := srv.Start(mux); err != nil {
		return fmt.Errorf("start asynq worker: %w", err)
	}
	c.Logger.Info("asynq worker started")

	<-ctx.Done()
	srv.Shutdown()
	return nil
}

// registerHandlers binds task types to their domain handlers. It is the single
// place each domain registers its consumers, e.g.:
//
//	mux.HandleFunc(infraasynq.TaskChannelDeliver, channelservice.HandleDeliver(...))
//
// The foundation registers none.
func registerHandlers(_ *asynq.ServeMux, _ *container.Container) {}

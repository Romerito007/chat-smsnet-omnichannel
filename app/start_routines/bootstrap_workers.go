package start_routines

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	chcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/contracts"
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
// place each domain registers its consumers.
func registerHandlers(mux *asynq.ServeMux, c *container.Container) {
	// automation.invoke: slow, non-critical invocation of the external flow.
	// The flow itself is external; this handler is the integration seam where the
	// outbound call + callback handling will live. For now it logs.
	mux.HandleFunc(infraasynq.TaskAutomationInvoke, func(ctx context.Context, t *asynq.Task) error {
		var p chcontracts.AutomationInvoke
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return err
		}
		c.Logger.Info("automation.invoke",
			"tenant_id", p.TenantID,
			"conversation_id", p.ConversationID,
			"integration_id", p.IntegrationID,
		)
		return nil
	})
}

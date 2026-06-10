package start_routines

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	autocontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/automation/contracts"
	chcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/contracts"
	ncontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/notifications/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	whcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/contracts"
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
	automation := factories.AutomationService(c)

	// automation.invoke: start the external flow for a new conversation.
	mux.HandleFunc(infraasynq.TaskAutomationInvoke, func(ctx context.Context, t *asynq.Task) error {
		var p chcontracts.AutomationInvoke
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return err
		}
		ctx = shared.WithTenant(ctx, p.TenantID)
		return automation.StartConversationAutomation(ctx, p.ConversationID, p.MessageID)
	})

	// automation.timeout: escalate a run still waiting for its callback.
	mux.HandleFunc(infraasynq.TaskAutomationTimeout, func(ctx context.Context, t *asynq.Task) error {
		var p autocontracts.TimeoutTask
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return err
		}
		ctx = shared.WithTenant(ctx, p.TenantID)
		return automation.HandleTimeout(ctx, p.RunID)
	})

	// channel.deliver / channel.retry: send an outbound message to the channel.
	outbound := factories.OutboundService(c)
	deliver := func(ctx context.Context, t *asynq.Task) error {
		var p chcontracts.DeliverTask
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return err
		}
		// The job carries its tenant; downstream repos are tenant-scoped.
		ctx = shared.WithTenant(ctx, p.TenantID)
		return outbound.Deliver(ctx, p.DeliveryID)
	}
	mux.HandleFunc(infraasynq.TaskChannelDeliver, deliver)
	mux.HandleFunc(infraasynq.TaskChannelRetry, deliver)

	// webhook.deliver / webhook.retry: deliver an outbound webhook with HMAC
	// signing, driving retry/backoff and dead-lettering.
	webhooks := factories.WebhookDeliveryService(c)
	deliverWebhook := func(ctx context.Context, t *asynq.Task) error {
		var p whcontracts.DeliverTask
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return err
		}
		ctx = shared.WithTenant(ctx, p.TenantID)
		return webhooks.Deliver(ctx, p.DeliveryID)
	}
	mux.HandleFunc(infraasynq.TaskWebhookDeliver, deliverWebhook)
	mux.HandleFunc(infraasynq.TaskWebhookRetry, deliverWebhook)

	// sla.check: scheduled, multi-tenant. Evaluates running SLA trackings across
	// all tenants, firing warnings/breaches (realtime + sla.breached webhook).
	sla := factories.SLAService(c)
	mux.HandleFunc(infraasynq.TaskSLACheck, func(ctx context.Context, _ *asynq.Task) error {
		return sla.RunCheck(ctx)
	})

	// notification.send: create the in-app notification + realtime, and enqueue an
	// email when the recipient's preference allows. notification.email: send the
	// privacy-safe email (subject + link only).
	notifications := factories.NotificationService(c)
	mux.HandleFunc(infraasynq.TaskNotificationSend, func(ctx context.Context, t *asynq.Task) error {
		var p ncontracts.SendTask
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return err
		}
		ctx = shared.WithTenant(ctx, p.TenantID)
		return notifications.Send(ctx, p)
	})
	mux.HandleFunc(infraasynq.TaskNotificationEmail, func(ctx context.Context, t *asynq.Task) error {
		var p ncontracts.EmailTask
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return err
		}
		ctx = authz.WithAuthContext(shared.WithTenant(ctx, p.TenantID), authz.SystemActor(p.TenantID))
		return notifications.SendEmail(ctx, p)
	})
}

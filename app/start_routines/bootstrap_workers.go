package start_routines

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	autocontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/automation/contracts"
	chcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/contracts"
	ccontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/csat/contracts"
	ncontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/notifications/contracts"
	pcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/privacy/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	tenantentity "github.com/romerito007/chat-smsnet-omnichannel/domain/tenant/entity"
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
	// The flow itself is external; this handler starts it and awaits the callback.
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

	// csat.send: deliver the survey question to the conversation's channel.
	// csat.expire: mark an unanswered survey expired. Both run as system actors.
	csat := factories.CSATService(c)
	mux.HandleFunc(infraasynq.TaskCSATSend, func(ctx context.Context, t *asynq.Task) error {
		var p ccontracts.SendTask
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return err
		}
		ctx = authz.WithAuthContext(shared.WithTenant(ctx, p.TenantID), authz.SystemActor(p.TenantID))
		return csat.Send(ctx, p)
	})
	mux.HandleFunc(infraasynq.TaskCSATExpire, func(ctx context.Context, t *asynq.Task) error {
		var p ccontracts.ExpireTask
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return err
		}
		ctx = authz.WithAuthContext(shared.WithTenant(ctx, p.TenantID), authz.SystemActor(p.TenantID))
		return csat.Expire(ctx, p)
	})

	// privacy.export: assemble a contact's data bundle into a file with a
	// temporary signed URL. Runs with the original requester as the audit actor.
	privacy := factories.PrivacyService(c)
	mux.HandleFunc(infraasynq.TaskPrivacyExport, func(ctx context.Context, t *asynq.Task) error {
		var p pcontracts.ExportTask
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return err
		}
		actor := authz.NewAuthContext(p.TenantID, p.ActorID, authz.AllPermissions(), nil, authz.ScopeAll)
		ctx = authz.WithAuthContext(shared.WithTenant(ctx, p.TenantID), actor)
		return privacy.RunExport(ctx, p.RequestID)
	})

	// Report export renders a real file synchronously and returns a signed URL at
	// request time (see domain/reports), so there is no async export job here.

	registerPeriodicHandlers(mux, c)
}

// registerPeriodicHandlers wires the scheduled, multi-tenant housekeeping jobs.
// Each fans work out across active tenants, runs the domain service as a system
// actor, and logs start/finish with the tenant count, item count and duration.
// All are idempotent.
func registerPeriodicHandlers(mux *asynq.ServeMux, c *container.Container) {
	tenants := factories.TenantRepository(c)
	conversations := factories.ConversationService(c)
	notifications := factories.NotificationService(c)
	connections := factories.ConnectionService(c)
	maint := factories.MaintenanceService(c)
	privacy := factories.PrivacyService(c)
	cfg := c.Config.Maintenance

	// chat.close_inactive_conversations: close conversations idle past the limit.
	mux.HandleFunc(infraasynq.TaskChatCloseInactive, func(ctx context.Context, _ *asynq.Task) error {
		return eachTenant(ctx, c, tenants, "chat.close_inactive_conversations", func(tctx context.Context, t *tenantentity.Tenant) (int, error) {
			idle := tenantDuration(t, "inactive_close_after_minutes", time.Minute, cfg.InactiveCloseAfter)
			return conversations.CloseInactive(tctx, idle)
		})
	})

	// notifications.cleanup: remove old read notifications.
	mux.HandleFunc(infraasynq.TaskNotificationCleanup, func(ctx context.Context, _ *asynq.Task) error {
		return eachTenant(ctx, c, tenants, "notifications.cleanup", func(tctx context.Context, t *tenantentity.Tenant) (int, error) {
			retention := tenantDuration(t, "notification_retention_days", 24*time.Hour, cfg.NotificationRetention)
			return notifications.Cleanup(tctx, time.Now().Add(-retention))
		})
	})

	// channels.health_check: probe connections and mark connected/error.
	mux.HandleFunc(infraasynq.TaskChannelsHealth, func(ctx context.Context, _ *asynq.Task) error {
		return eachTenant(ctx, c, tenants, "channels.health_check", func(tctx context.Context, _ *tenantentity.Tenant) (int, error) {
			return connections.HealthCheck(tctx)
		})
	})

	// audit.compact: enforce audit-log retention.
	mux.HandleFunc(infraasynq.TaskAuditCompact, func(ctx context.Context, _ *asynq.Task) error {
		return eachTenant(ctx, c, tenants, "audit.compact", func(tctx context.Context, t *tenantentity.Tenant) (int, error) {
			retention := tenantDuration(t, "audit_retention_days", 24*time.Hour, cfg.AuditRetention)
			return maint.CompactAudit(tctx, retention)
		})
	})

	// privacy.retention: apply each tenant's RetentionPolicy, deleting data past
	// the configured cutoffs while skipping anything under an active legal hold.
	mux.HandleFunc(infraasynq.TaskPrivacyRetention, func(ctx context.Context, _ *asynq.Task) error {
		return eachTenant(ctx, c, tenants, "privacy.retention", func(tctx context.Context, _ *tenantentity.Tenant) (int, error) {
			return privacy.ApplyRetention(tctx)
		})
	})

	// reports.snapshot: pre-aggregate per-tenant daily metrics.
	mux.HandleFunc(infraasynq.TaskReportsSnapshot, func(ctx context.Context, _ *asynq.Task) error {
		return eachTenant(ctx, c, tenants, "reports.snapshot", func(tctx context.Context, _ *tenantentity.Tenant) (int, error) {
			if _, err := maint.SnapshotDay(tctx, time.Now()); err != nil {
				return 0, err
			}
			return 1, nil
		})
	})
}

// eachTenant fans a job out across active tenants as a system actor, logging
// start/finish with counts and duration. Per-tenant failures are logged and
// skipped so one tenant never blocks the rest.
func eachTenant(ctx context.Context, c *container.Container, tenants tenantRepo, job string, fn func(context.Context, *tenantentity.Tenant) (int, error)) error {
	start := time.Now()
	c.Logger.Info("periodic job started", "job", job)
	list, err := tenants.ListActive(ctx)
	if err != nil {
		c.Logger.Error("periodic job: list tenants failed", "job", job, "error", err)
		return err
	}
	total := 0
	for _, t := range list {
		tctx := authz.WithAuthContext(shared.WithTenant(ctx, t.ID), authz.SystemActor(t.ID))
		n, ferr := fn(tctx, t)
		if ferr != nil {
			c.Logger.Error("periodic job: tenant failed", "job", job, "tenant", t.ID, "error", ferr)
			continue
		}
		total += n
	}
	c.Logger.Info("periodic job finished", "job", job,
		"tenants", len(list), "count", total, "duration_ms", time.Since(start).Milliseconds())
	return nil
}

// tenantRepo is the minimal tenant-listing dependency used by the periodic jobs.
type tenantRepo interface {
	ListActive(ctx context.Context) ([]*tenantentity.Tenant, error)
}

// tenantDuration reads a per-tenant numeric override from settings (multiplied by
// unit), falling back to the default.
func tenantDuration(t *tenantentity.Tenant, key string, unit, def time.Duration) time.Duration {
	if t != nil && t.Settings != nil {
		if v, ok := t.Settings[key]; ok {
			if n := toFloat(v); n > 0 {
				return time.Duration(n * float64(unit))
			}
		}
	}
	return def
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int32:
		return float64(n)
	case int64:
		return float64(n)
	default:
		return 0
	}
}

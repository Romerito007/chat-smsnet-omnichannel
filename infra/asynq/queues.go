// Package asynq wraps the hibiken/asynq client and server: queue names, the
// enqueue client, the worker server and job middleware. Task payloads are
// defined per domain in domain/<X>/contracts/tasks.go and handlers are wired in
// app/start_routines/bootstrap_workers.go.
package asynq

// Queue names mirror section 4 of the architecture document.
const (
	QueueCritical = "critical"
	QueueDefault  = "default"
	QueueChannels = "channels"
	QueueWebhooks = "webhooks"
	QueueAI       = "ai"
	QueueReports  = "reports"
)

// Task type identifiers. Centralized here so producers and consumers agree on
// the wire name. Payload structs live in each domain's contracts/tasks.go.
const (
	// Outbound message delivery is no longer a separate channel job: an agent's
	// reply is delivered through the message_created webhook (webhook.deliver).

	TaskWebhookDeliver = "webhook.deliver"
	TaskWebhookRetry   = "webhook.retry"

	TaskNotificationSend    = "notification.send"
	TaskNotificationEmail   = "notification.email"
	TaskNotificationCleanup = "notifications.cleanup"

	TaskAISuggest   = "ai.suggest"
	TaskAISummarize = "ai.summarize"
	TaskAIClassify  = "ai.classify"

	TaskCSATSend   = "csat.send"
	TaskCSATExpire = "csat.expire"

	TaskAttachmentProcess = "attachment.process"

	// Scheduler tasks.
	TaskChatCloseInactive = "chat.close_inactive_conversations"
	TaskSLACheck          = "sla.check"
	TaskReportsSnapshot   = "reports.snapshot"
	TaskAuditCompact      = "audit.compact"
	TaskChannelsHealth    = "channels.health_check"

	// Privacy (LGPD) tasks. privacy.export assembles a contact's data bundle into
	// a file with a signed URL; privacy.retention applies the per-tenant
	// RetentionPolicy across the scheduled, multi-tenant fan-out.
	TaskPrivacyExport    = "privacy.export"
	TaskPrivacyRetention = "privacy.retention"

	// TaskRuleEvaluate evaluates the tenant's automation rules for one conversation
	// event, off the hot path of the emitting service.
	TaskRuleEvaluate = "automationrule.evaluate"
)

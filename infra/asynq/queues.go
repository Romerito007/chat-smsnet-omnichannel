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
	TaskChannelDeliver = "channel.deliver"
	TaskChannelRetry   = "channel.retry"

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

	TaskAutomationInvoke  = "automation.invoke"
	TaskAutomationTimeout = "automation.timeout"

	// Scheduler tasks.
	TaskChatCloseInactive = "chat.close_inactive_conversations"
	TaskSLACheck          = "sla.check"
	TaskReportsSnapshot   = "reports.snapshot"
	// TaskReportsExport is prepared for a future async report export (CSV/XLSX to
	// object storage + a download link). Not implemented in the MVP.
	TaskReportsExport  = "reports.export"
	TaskAuditCompact   = "audit.compact"
	TaskChannelsHealth = "channels.health_check"

	// Privacy (LGPD) tasks. privacy.export assembles a contact's data bundle into
	// a file with a signed URL; privacy.retention applies the per-tenant
	// RetentionPolicy across the scheduled, multi-tenant fan-out.
	TaskPrivacyExport    = "privacy.export"
	TaskPrivacyRetention = "privacy.retention"
)

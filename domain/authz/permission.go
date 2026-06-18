package authz

// Permission is a fine-grained capability checked by the authorization
// middleware and services. The format is "<resource>.<action>".
type Permission string

// Permission catalog. This is the single source of truth for the permission
// vocabulary; roles reference these by value.
const (
	ConversationRead     Permission = "conversation.read"
	ConversationAssign   Permission = "conversation.assign"
	ConversationTransfer Permission = "conversation.transfer"
	ConversationClose    Permission = "conversation.close"

	MessageSend         Permission = "message.send"
	MessageInternalNote Permission = "message.internal_note"
	MessageDelete       Permission = "message.delete"

	ContactRead                 Permission = "contact.read"
	ContactWrite                Permission = "contact.write"
	ContactViewFinancial        Permission = "contact.view_financial"
	ContactViewConnectionStatus Permission = "contact.view_connection_status"

	SectorManage Permission = "sector.manage"
	QueueManage  Permission = "queue.manage"
	UserManage   Permission = "user.manage"

	AutomationManage Permission = "automation.manage"

	CopilotUse       Permission = "copilot.use"
	CopilotConfigure Permission = "copilot.configure"

	IntegrationRead          Permission = "integration.read"
	IntegrationConfigure     Permission = "integration.configure"
	IntegrationExecuteAction Permission = "integration.execute_action"

	ChannelManage Permission = "channel.manage"
	WebhookManage Permission = "webhook.manage"

	// GroupView lets an agent list/search the known WhatsApp groups; GroupManage
	// lets a supervisor/admin toggle the attend filter and trigger a group sync.
	GroupView   Permission = "group.view"
	GroupManage Permission = "group.manage"

	// PipelineView lets a sales user read the funnels; PipelineManage lets an
	// admin configure the pipelines and their stages.
	PipelineView   Permission = "pipeline.view"
	PipelineManage Permission = "pipeline.manage"

	ReportView   Permission = "report.view"
	ReportExport Permission = "report.export"

	AuditView     Permission = "audit.view"
	PrivacyManage Permission = "privacy.manage"

	CustomAttributeManage Permission = "customattribute.manage"
)

// AllPermissions is the ordered catalog of every known permission. Used by the
// seed (owner role) and to validate role definitions.
func AllPermissions() []Permission {
	return []Permission{
		ConversationRead, ConversationAssign, ConversationTransfer, ConversationClose,
		MessageSend, MessageInternalNote, MessageDelete,
		ContactRead, ContactWrite, ContactViewFinancial, ContactViewConnectionStatus,
		SectorManage, QueueManage, UserManage,
		AutomationManage,
		CopilotUse, CopilotConfigure,
		IntegrationRead, IntegrationConfigure, IntegrationExecuteAction,
		ChannelManage, WebhookManage,
		GroupView, GroupManage,
		PipelineView, PipelineManage,
		ReportView, ReportExport,
		AuditView, PrivacyManage,
		CustomAttributeManage,
	}
}

// validPermissions is the lookup set built once from the catalog.
var validPermissions = func() map[Permission]struct{} {
	m := make(map[Permission]struct{}, len(AllPermissions()))
	for _, p := range AllPermissions() {
		m[p] = struct{}{}
	}
	return m
}()

// IsValid reports whether p is a known permission.
func IsValid(p Permission) bool {
	_, ok := validPermissions[p]
	return ok
}

// SanitizePermissions returns the subset of perms that are valid, de-duplicated
// and in catalog order. Unknown permissions are dropped.
func SanitizePermissions(perms []Permission) []Permission {
	seen := make(map[Permission]struct{}, len(perms))
	for _, p := range perms {
		if IsValid(p) {
			seen[p] = struct{}{}
		}
	}
	out := make([]Permission, 0, len(seen))
	for _, p := range AllPermissions() {
		if _, ok := seen[p]; ok {
			out = append(out, p)
		}
	}
	return out
}

// Package authz holds the permission vocabulary, the resolved authorization
// state (AuthContext) and the default role definitions used to seed a tenant.
// It depends only on the standard library so it sits at the bottom of the graph.
package authz

// DefaultRole names the roles created when a tenant is seeded.
const (
	DefaultRoleOwner = "owner"
	DefaultRoleAdmin = "admin"
	DefaultRoleAgent = "agent"
)

// DefaultRoleDefinition describes a seeded role: its permissions and sector
// scope. Owner receives the full catalog; admin a broad subset; agent the
// day-to-day attendance permissions restricted to its own sectors.
type DefaultRoleDefinition struct {
	Name        string
	Permissions []Permission
	SectorScope SectorScope
}

// DefaultRoles returns the idempotent role seed for a tenant.
func DefaultRoles() []DefaultRoleDefinition {
	return []DefaultRoleDefinition{
		{
			Name:        DefaultRoleOwner,
			Permissions: AllPermissions(),
			SectorScope: ScopeAll,
		},
		{
			Name: DefaultRoleAdmin,
			Permissions: []Permission{
				ConversationRead, ConversationAssign, ConversationTransfer, ConversationClose,
				MessageSend, MessageInternalNote, MessageDelete,
				ContactRead, ContactWrite, ContactViewConnectionStatus,
				SectorManage, QueueManage, UserManage,
				AutomationManage, CopilotUse, CopilotConfigure,
				IntegrationRead, IntegrationConfigure,
				ChannelManage, WebhookManage,
				PipelineView, PipelineManage,
				CustomAttributeManage,
				ReportView, ReportExport, AuditView,
			},
			SectorScope: ScopeAll,
		},
		{
			Name: DefaultRoleAgent,
			Permissions: []Permission{
				ConversationRead, ConversationAssign, ConversationClose,
				MessageSend, MessageInternalNote,
				ContactRead, ContactWrite,
				CopilotUse,
			},
			SectorScope: ScopeOwn,
		},
	}
}

package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	mcpservice "github.com/romerito007/chat-smsnet-omnichannel/domain/mcp/service"
	convrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/conversations"
	mcprepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/mcp"
	inframcp "github.com/romerito007/chat-smsnet-omnichannel/infra/mcp"
	mcpctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/mcp"
)

// mcpClient builds the shared Streamable HTTP MCP client.
func mcpClient() *inframcp.Client { return inframcp.NewClient() }

// MCPServerService builds the MCP server registration/discovery service. The
// cipher encrypts the per-server auth token at rest.
func MCPServerService(c *container.Container) *mcpservice.ServerService {
	svc := mcpservice.NewServerService(mcprepo.NewServerRepository(c.Mongo.DB, c.Cipher), mcpClient(), clock)
	svc.SetAuditor(AuditService(c))
	return svc
}

// MCPToolService builds the tool execution + approval service (also the copilot
// tool broker).
func MCPToolService(c *container.Container) *mcpservice.ToolService {
	svc := mcpservice.NewToolService(
		mcprepo.NewServerRepository(c.Mongo.DB, c.Cipher),
		mcprepo.NewApprovalRepository(c.Mongo.DB),
		mcprepo.NewCallLogRepository(c.Mongo.DB),
		convrepo.NewConversationRepository(c.Mongo.DB),
		mcpClient(),
		c.Events,
		clock,
	)
	svc.SetAuditor(AuditService(c))
	return svc
}

// MCPServerController builds the server config controller.
func MCPServerController(c *container.Container) *mcpctl.ServerController {
	return mcpctl.NewServerController(MCPServerService(c))
}

// MCPToolController builds the conversation-scoped tool controller.
func MCPToolController(c *container.Container) *mcpctl.ToolController {
	return mcpctl.NewToolController(MCPToolService(c))
}

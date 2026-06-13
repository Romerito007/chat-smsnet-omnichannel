package openapi

// This file registers every /v1 operation, grouped by domain. Each group maps
// the real routes (app/routes/http/*_routes.go) to operations with the real
// request/response schemas, the standard error envelope and Bearer security.

func registerAuth(p *paths) {
	pub := func(summary string, in M, out M, code string) M {
		return op(opConfig{tag: "auth", summary: summary, public: true, reqBody: body(in),
			responses: M{code: jsonResp("OK", out)}})
	}
	p.add("POST", "/v1/auth/login", pub("Log in with email + password", ref("LoginRequest"), ref("TokenResponse"), "200"))
	p.add("POST", "/v1/auth/refresh", pub("Rotate the refresh token", ref("RefreshRequest"), ref("TokenResponse"), "200"))
	p.add("POST", "/v1/auth/signup", pub("Self-service company signup (neutral response)", ref("SignupRequest"), ref("MessageAck"), "202"))
	p.add("POST", "/v1/auth/forgot-password", pub("Request a password reset (neutral)", ref("EmailRequest"), ref("MessageAck"), "202"))
	p.add("POST", "/v1/auth/resend-verification", pub("Resend the verification email (neutral)", ref("EmailRequest"), ref("MessageAck"), "202"))
	p.add("POST", "/v1/auth/verify-email", pub("Confirm an email address", ref("VerifyEmailRequest"), ref("MessageAck"), "200"))
	p.add("POST", "/v1/auth/reset-password", pub("Reset the password with a token", ref("ResetPasswordRequest"), ref("MessageAck"), "200"))
	p.add("POST", "/v1/auth/accept-invite", pub("Accept a teammate invitation", ref("AcceptInviteRequest"), ref("MessageAck"), "201"))

	p.add("POST", "/v1/auth/logout", op(opConfig{tag: "auth", summary: "Revoke the current refresh token",
		reqBody: body(ref("LogoutRequest")), responses: M{"204": emptyResp("Logged out")}}))
	p.add("GET", "/v1/me", op(opConfig{tag: "auth", summary: "Current user + effective permissions",
		responses: M{"200": jsonResp("Identity", ref("MeResponse"))}}))
	p.add("PATCH", "/v1/me", op(opConfig{tag: "auth", summary: "Update own profile",
		reqBody: body(ref("UpdateProfileRequest")), responses: M{"200": jsonResp("Updated", ref("User"))}}))
	p.add("POST", "/v1/me/change-password", op(opConfig{tag: "auth", summary: "Change own password",
		reqBody: body(ref("ChangePasswordRequest")), responses: M{"204": emptyResp("Changed")}}))
}

func registerTenantIAM(p *paths) {
	p.add("GET", "/v1/tenants/current", op(opConfig{tag: "tenant", summary: "Get the current tenant",
		responses: M{"200": jsonResp("Tenant", ref("Tenant"))}}))
	p.add("PATCH", "/v1/tenants/current", op(opConfig{tag: "tenant", summary: "Update the current tenant",
		reqBody: body(ref("UpdateTenantRequest")), responses: M{"200": jsonResp("Updated", ref("Tenant"))}}))

	p.crud("/v1/users", "iam", "user", ref("User"), ref("CreateUserRequest"), ref("UpdateUserRequest"))
	p.add("POST", "/v1/users/invite", op(opConfig{tag: "iam", summary: "Invite a teammate",
		reqBody: body(ref("InviteUserRequest")), responses: M{"201": jsonResp("Invitation", ref("InviteResponse"))}}))
	p.crud("/v1/roles", "iam", "role", ref("Role"), ref("CreateRoleRequest"), ref("UpdateRoleRequest"))
}

func registerOrg(p *paths) {
	p.crud("/v1/sectors", "sectors", "sector", ref("Sector"), ref("CreateSectorRequest"), ref("UpdateSectorRequest"))
	p.crud("/v1/queues", "queues", "queue", ref("Queue"), ref("CreateQueueRequest"), ref("UpdateQueueRequest"))

	p.add("GET", "/v1/agents/presence", op(opConfig{tag: "presence", summary: "List agent presence",
		params:    []M{queryParam("sector_id", "Scope to the agents of this sector (server-side); omit for the whole team.")},
		responses: M{"200": jsonResp("Presence list", arr(ref("Presence")))}}))
	p.add("POST", "/v1/agents/presence/status", op(opConfig{tag: "presence", summary: "Set own (or an agent's) status",
		reqBody: body(ref("SetStatusRequest")), responses: M{"200": jsonResp("Presence", ref("Presence"))}}))
	p.add("GET", "/v1/agents", op(opConfig{tag: "presence",
		summary:   "List assignable agents (id, name, presence) for the assignment selector — conversation.assign",
		params:    []M{queryParam("sector_id", "Only agents assignable to this sector (matches what assign accepts).")},
		responses: M{"200": jsonResp("Assignable agents", dataArr(ref("AssignableAgent")))}}))

	p.add("GET", "/v1/sectors/{id}/business-status", op(opConfig{tag: "businesshours", summary: "Sector open/closed status",
		params: []M{pathParam("id", "sector id")}, responses: M{"200": jsonResp("Business status", ref("BusinessStatus"))}}))
	p.crud("/v1/holidays", "businesshours", "holiday", ref("Holiday"), ref("CreateHolidayRequest"), ref("UpdateHolidayRequest"))
}

func registerConversations(p *paths) {
	p.add("GET", "/v1/conversations", op(opConfig{tag: "conversations", summary: "List conversations",
		params: append(paginationParams(),
			M{"name": "status", "in": "query", "required": false, "description": "Filter by status (exact match; same vocabulary as the PATCH body).", "schema": conversationStatusEnum()},
			queryParam("assigned_to", "Filter by agent"), queryParam("sector_id", "Filter by sector"),
			queryParam("contact_id", "Filter by contact (the contact's conversation history; paginated, last_message embedded)")),
		responses: M{"200": jsonResp("Conversation page", pageOf(ref("Conversation")))}}))
	p.add("POST", "/v1/conversations", op(opConfig{tag: "conversations", summary: "Create a conversation",
		reqBody: body(ref("CreateConversationRequest")), responses: M{"201": jsonResp("Created", ref("Conversation"))}}))
	idp := []M{pathParam("id", "conversation id")}
	p.add("GET", "/v1/conversations/{id}", op(opConfig{tag: "conversations", summary: "Get a conversation",
		params: idp, responses: M{"200": jsonResp("Conversation", ref("Conversation")), "404": respRef("Error404")}}))
	p.add("PATCH", "/v1/conversations/{id}", op(opConfig{tag: "conversations", summary: "Update a conversation",
		params: idp, reqBody: body(ref("UpdateConversationRequest")), responses: M{"200": jsonResp("Updated", ref("Conversation"))}}))
	p.add("GET", "/v1/conversations/{id}/messages", op(opConfig{tag: "conversations", summary: "List messages",
		params: append([]M{pathParam("id", "conversation id")}, paginationParams()...), responses: M{"200": jsonResp("Message page", pageOf(ref("Message")))}}))
	p.add("GET", "/v1/conversations/{id}/events", op(opConfig{tag: "conversations", summary: "List the lifecycle/automation timeline (separate from chat messages)",
		params: append([]M{pathParam("id", "conversation id")}, paginationParams()...), responses: M{"200": jsonResp("Event page", pageOf(ref("ConversationEvent")))}}))
	p.add("POST", "/v1/conversations/{id}/messages", op(opConfig{tag: "conversations", summary: "Send a message",
		params: idp, reqBody: body(ref("SendMessageRequest")), responses: M{"201": jsonResp("Sent", ref("Message"))}}))
	midp := []M{pathParam("id", "conversation id"), pathParam("mid", "message id")}
	p.add("PATCH", "/v1/conversations/{id}/messages/{mid}", op(opConfig{tag: "conversations", summary: "Edit a message",
		params: midp, reqBody: body(ref("EditMessageRequest")), responses: M{"200": jsonResp("Edited", ref("Message"))}}))
	p.add("DELETE", "/v1/conversations/{id}/messages/{mid}", op(opConfig{tag: "conversations", summary: "Delete a message",
		params: midp, responses: M{"204": emptyResp("Deleted")}}))
	p.add("POST", "/v1/conversations/{id}/internal-notes", op(opConfig{tag: "conversations", summary: "Add an internal note",
		params: idp, reqBody: body(ref("InternalNoteRequest")), responses: M{"201": jsonResp("Note", ref("Message"))}}))
	p.add("POST", "/v1/conversations/{id}/close", op(opConfig{tag: "conversations", summary: "Close a conversation",
		params: idp, reqBody: body(ref("CloseRequest")), responses: M{"200": jsonResp("Closed", ref("Conversation"))}}))
	p.add("POST", "/v1/conversations/{id}/reopen", op(opConfig{tag: "conversations", summary: "Reopen a conversation",
		params: idp, responses: M{"200": jsonResp("Reopened", ref("Conversation"))}}))
	p.add("POST", "/v1/conversations/{id}/typing/start", op(opConfig{tag: "conversations", summary: "Signal typing start",
		params: idp, responses: M{"204": emptyResp("OK")}}))
	p.add("POST", "/v1/conversations/{id}/typing/stop", op(opConfig{tag: "conversations", summary: "Signal typing stop",
		params: idp, responses: M{"204": emptyResp("OK")}}))
	p.add("POST", "/v1/conversations/{id}/read", op(opConfig{tag: "conversations", summary: "Mark conversation read",
		params: idp, responses: M{"204": emptyResp("OK")}}))

	// routing
	p.add("POST", "/v1/conversations/{id}/assign", op(opConfig{tag: "routing", summary: "Assign to an agent",
		params: idp, reqBody: body(ref("AssignRequest")), responses: M{"200": jsonResp("Assigned", ref("Conversation"))}}))
	p.add("POST", "/v1/conversations/{id}/transfer", op(opConfig{tag: "routing", summary: "Transfer to a sector/agent",
		params: idp, reqBody: body(ref("TransferRequest")), responses: M{"200": jsonResp("Transferred", ref("Conversation"))}}))
	p.add("POST", "/v1/conversations/{id}/enqueue", op(opConfig{tag: "routing", summary: "Enqueue for distribution",
		params: idp, reqBody: body(ref("EnqueueRequest")), responses: M{"200": jsonResp("Enqueued", ref("Conversation"))}}))
	p.add("POST", "/v1/routing/run", op(opConfig{tag: "routing", summary: "Run distribution for a conversation",
		reqBody: body(ref("RoutingRunRequest")), responses: M{"200": jsonResp("Routed", ref("Conversation"))}}))

	// conversation tags
	p.add("POST", "/v1/conversations/{id}/tags", op(opConfig{tag: "conversationtools", summary: "Apply/remove conversation tags",
		params: idp, reqBody: body(ref("ApplyTagsRequest")), responses: M{"200": jsonResp("Updated", ref("Conversation"))}}))

	// conversation SLA
	p.add("GET", "/v1/conversations/{id}/sla", op(opConfig{tag: "sla", summary: "SLA tracking for a conversation",
		params: idp, responses: M{"200": jsonResp("SLA tracking, or null when the conversation has no tracking yet",
			M{"oneOf": []any{ref("SLATracking"), M{"type": "null"}}})}}))

	// contacts (CRM)
	p.add("GET", "/v1/contacts", op(opConfig{tag: "contacts", summary: "List contacts (contact.read)",
		params: append(paginationParams(),
			queryParam("q", "Free-text filter (name/phone/document/email)"),
			queryParam("name", "Filter by name (case-insensitive substring)"),
			queryParam("phone", "Filter by phone (substring of any phone)"),
			queryParam("tag_id", "Filter by tag id (contacts carrying this tag). Combines with the others (AND).")),
		responses: M{"200": jsonResp("Contact page", pageOf(ref("Contact")))}}))
	p.add("POST", "/v1/contacts", op(opConfig{tag: "contacts", summary: "Create a contact (contact.write)",
		reqBody:   body(ref("CreateContactRequest")),
		responses: M{"201": jsonResp("Created", ref("Contact")), "409": respRef("Error409")}}))
	p.add("GET", "/v1/contacts/{id}", op(opConfig{tag: "contacts", summary: "Get a contact (contact.read, tenant-scoped)",
		params: []M{pathParam("id", "contact id")}, responses: M{"200": jsonResp("Contact", ref("Contact")), "404": respRef("Error404")}}))
	p.add("PATCH", "/v1/contacts/{id}", op(opConfig{tag: "contacts", summary: "Update a contact (contact.write, partial)",
		params: []M{pathParam("id", "contact id")}, reqBody: body(ref("UpdateContactRequest")),
		responses: M{"200": jsonResp("Updated", ref("Contact")), "404": respRef("Error404"), "409": respRef("Error409")}}))
}

func registerChannels(p *paths) {
	p.add("GET", "/v1/channels", op(opConfig{tag: "channels", summary: "List channel connections", params: paginationParams(),
		responses: M{"200": jsonResp("Channel page", pageOf(ref("Channel")))}}))
	p.add("POST", "/v1/channels", op(opConfig{tag: "channels", summary: "Create a channel connection (returns secrets once)",
		reqBody: body(ref("CreateChannelRequest")), responses: M{"201": jsonResp("Created", ref("ChannelCreated"))}}))
	idp := []M{pathParam("id", "channel id")}
	p.add("GET", "/v1/channels/{id}", op(opConfig{tag: "channels", summary: "Get a channel connection", params: idp,
		responses: M{"200": jsonResp("Channel", ref("Channel")), "404": respRef("Error404")}}))
	p.add("PATCH", "/v1/channels/{id}", op(opConfig{tag: "channels", summary: "Update a channel connection", params: idp,
		reqBody: body(ref("UpdateChannelRequest")), responses: M{"200": jsonResp("Updated", ref("Channel"))}}))
	p.add("DELETE", "/v1/channels/{id}", op(opConfig{tag: "channels", summary: "Delete a channel connection", params: idp,
		responses: M{"204": emptyResp("Deleted")}}))
	p.add("POST", "/v1/channels/{id}/test", op(opConfig{tag: "channels", summary: "Test outbound delivery", params: idp,
		responses: M{"200": jsonResp("Result", ref("TestResult"))}}))
	p.add("POST", "/v1/channels/{id}/rotate-inbound-token", op(opConfig{tag: "channels",
		summary:   "Rotate the channel integration token (revokes the prior one; returned once)",
		params:    idp,
		responses: M{"200": jsonResp("Rotated", ref("RotatedInboundToken")), "404": respRef("Error404")}}))

	// Public inbound endpoints authenticate with the channel integration token via
	// the X-Inbound-Token header (preferred) or an inbound_token body field — never
	// the front's Bearer JWT.
	inboundParams := []M{pathParam("channel", "channel type"),
		headerParam("X-Inbound-Token", "Channel integration token (preferred over the inbound_token body field).")}
	// Chatwoot-compatible: accepts JSON (media by URL) OR multipart/form-data (raw
	// file attachments, like Chatwoot's create-message API).
	inboundBody := M{"required": true, "content": M{
		"application/json":    M{"schema": ref("InboundMessageRequest")},
		"multipart/form-data": M{"schema": ref("InboundMessageMultipart"), "encoding": M{"attachments[]": M{"contentType": "image/*, audio/*, video/*, application/*"}}},
	}}
	p.add("POST", "/v1/inbound/channel/{channel}/messages", op(opConfig{tag: "channels", summary: "Ingest an inbound message (channel-authenticated; JSON or Chatwoot multipart)",
		public: true, params: inboundParams, reqBody: inboundBody, responses: M{"200": jsonResp("Accepted", ref("TestResult"))}}))
	p.add("POST", "/v1/inbound/channel/{channel}/delivery-receipts", op(opConfig{tag: "channels", summary: "Ingest delivery receipts (channel-authenticated)",
		public: true, params: inboundParams, reqBody: body(freeObject()), responses: M{"200": jsonResp("Accepted", freeObject())}}))
}

func registerIntegrations(p *paths) {
	// automation
	p.crud("/v1/automation/integrations", "automation", "automation integration", ref("AutomationIntegration"), ref("CreateAutomationRequest"), ref("UpdateAutomationRequest"))
	p.add("GET", "/v1/automation/runs", op(opConfig{tag: "automation", summary: "List automation runs", params: paginationParams(),
		responses: M{"200": jsonResp("Run page", pageOf(ref("AutomationRun")))}}))
	p.add("GET", "/v1/automation/runs/{id}", op(opConfig{tag: "automation", summary: "Get an automation run",
		params: []M{pathParam("id", "run id")}, responses: M{"200": jsonResp("Run", ref("AutomationRun"))}}))
	p.add("POST", "/v1/automation/callbacks/{tenant_id}", op(opConfig{tag: "automation", summary: "External automation callback (signed)",
		public: true, params: []M{pathParam("tenant_id", "tenant id")}, reqBody: body(freeObject()), responses: M{"200": jsonResp("Accepted", freeObject())}}))

	// providerhub: catalog, gateway status and ISP profiles (many per tenant)
	p.add("GET", "/v1/providerhub/catalog", op(opConfig{tag: "providerhub", summary: "Supported ISP catalog (credential fields + actions per ISP)",
		responses: M{"200": jsonResp("Catalog", ref("ProviderHubCatalog"))}}))
	p.add("GET", "/v1/providerhub/config", op(opConfig{tag: "providerhub", summary: "SMSNET gateway status + ISP-profile summary",
		responses: M{"200": jsonResp("Gateway status", ref("ProviderHubGatewayStatus"))}}))
	p.add("GET", "/v1/providerhub/profiles", op(opConfig{tag: "providerhub", summary: "List ISP profiles",
		responses: M{"200": jsonResp("ISP profiles", dataArr(ref("ISPProfile")))}}))
	p.add("POST", "/v1/providerhub/profiles", op(opConfig{tag: "providerhub", summary: "Create an ISP profile",
		reqBody: body(ref("CreateISPProfileRequest")), responses: M{"201": jsonResp("Created", ref("ISPProfile"))}}))
	profileIDP := []M{pathParam("id", "ISP profile id")}
	p.add("GET", "/v1/providerhub/profiles/{id}", op(opConfig{tag: "providerhub", summary: "Get an ISP profile",
		params: profileIDP, responses: M{"200": jsonResp("ISP profile", ref("ISPProfile")), "404": errorResponse("Not found.")}}))
	p.add("PATCH", "/v1/providerhub/profiles/{id}", op(opConfig{tag: "providerhub", summary: "Update an ISP profile",
		params: profileIDP, reqBody: body(ref("UpdateISPProfileRequest")), responses: M{"200": jsonResp("Updated", ref("ISPProfile")), "404": errorResponse("Not found.")}}))
	p.add("DELETE", "/v1/providerhub/profiles/{id}", op(opConfig{tag: "providerhub", summary: "Delete an ISP profile",
		params: profileIDP, responses: M{"204": emptyResp("Deleted"), "404": errorResponse("Not found.")}}))
	p.add("POST", "/v1/providerhub/profiles/{id}/default", op(opConfig{tag: "providerhub", summary: "Make this the default ISP profile",
		params: profileIDP, responses: M{"200": jsonResp("Updated", ref("ISPProfile")), "404": errorResponse("Not found.")}}))
	p.add("POST", "/v1/providerhub/profiles/{id}/test", op(opConfig{tag: "providerhub", summary: "Test an ISP profile against the gateway",
		params: profileIDP, responses: M{"200": jsonResp("Result", ref("ISPProfileTestResult")), "404": errorResponse("Not found.")}}))

	// external on-demand queries (smsnet-integrations) under a conversation. Reads
	// are POST so the body can carry isp_config_id. When the ISP profile is
	// ambiguous (no default, 2+ eligible) any of these returns 200 with a
	// NeedsISPSelection body instead of the normal payload.
	cidp := []M{pathParam("id", "conversation id")}
	idemHeader := headerParam("Idempotency-Key", "Idempotency key for the side-effect call; replayed on retry and forwarded to the gateway. Generated server-side when omitted.")
	orSelect := func(schema M) M { return M{"oneOf": []any{schema, ref("NeedsISPSelection")}} }
	p.add("POST", "/v1/conversations/{id}/external/cliente", op(opConfig{tag: "providerhub", summary: "Customer lookup (read); may need a contract or ISP selection",
		params: cidp, reqBody: body(ref("ClienteRequest")),
		responses: M{"200": jsonResp("Customer, a contract-selection prompt, or an ISP-selection prompt", orSelect(ref("ClienteResult")))}}))
	p.add("POST", "/v1/conversations/{id}/external/planos", op(opConfig{tag: "providerhub", summary: "Plans (read)",
		params: cidp, reqBody: body(ref("ISPSelectorRequest")), responses: M{"200": jsonResp("Plans (or an ISP-selection prompt)", orSelect(ref("PlanosResult")))}}))
	p.add("POST", "/v1/conversations/{id}/external/empresa", op(opConfig{tag: "providerhub", summary: "Company info (read)",
		params: cidp, reqBody: body(ref("ISPSelectorRequest")), responses: M{"200": jsonResp("Company (or an ISP-selection prompt)", orSelect(ref("Empresa")))}}))
	p.add("POST", "/v1/conversations/{id}/external/liberacao", op(opConfig{tag: "providerhub", summary: "Trust-release a customer (write)",
		params: append([]M{pathParam("id", "conversation id")}, idemHeader), reqBody: body(ref("LiberacaoRequest")), responses: M{"200": jsonResp("Result (or an ISP-selection prompt)", orSelect(ref("Liberacao")))}}))
	p.add("POST", "/v1/conversations/{id}/external/chamado", op(opConfig{tag: "providerhub", summary: "Open a support ticket (write)",
		params: append([]M{pathParam("id", "conversation id")}, idemHeader), reqBody: body(ref("ChamadoRequest")), responses: M{"200": jsonResp("Result (or an ISP-selection prompt)", orSelect(ref("Chamado")))}}))

	// webhooks
	p.add("GET", "/v1/webhooks", op(opConfig{tag: "webhooks", summary: "List webhook subscriptions", params: paginationParams(),
		responses: M{"200": jsonResp("Webhook page", pageOf(ref("Webhook")))}}))
	p.add("POST", "/v1/webhooks", op(opConfig{tag: "webhooks", summary: "Create a webhook (returns the secret once)",
		reqBody: body(ref("CreateWebhookRequest")), responses: M{"201": jsonResp("Created", ref("WebhookCreated"))}}))
	widp := []M{pathParam("id", "webhook id")}
	p.add("GET", "/v1/webhooks/{id}", op(opConfig{tag: "webhooks", summary: "Get a webhook", params: widp,
		responses: M{"200": jsonResp("Webhook", ref("Webhook")), "404": respRef("Error404")}}))
	p.add("PATCH", "/v1/webhooks/{id}", op(opConfig{tag: "webhooks", summary: "Update a webhook", params: widp,
		reqBody: body(ref("UpdateWebhookRequest")), responses: M{"200": jsonResp("Updated", ref("Webhook"))}}))
	p.add("DELETE", "/v1/webhooks/{id}", op(opConfig{tag: "webhooks", summary: "Delete a webhook", params: widp,
		responses: M{"204": emptyResp("Deleted")}}))
	p.add("POST", "/v1/webhooks/{id}/test", op(opConfig{tag: "webhooks", summary: "Send a test delivery", params: widp,
		responses: M{"200": jsonResp("Result", ref("TestResult"))}}))
	p.add("GET", "/v1/webhooks/{id}/deliveries", op(opConfig{tag: "webhooks", summary: "List recent deliveries",
		params: append([]M{pathParam("id", "webhook id")}, paginationParams()...), responses: M{"200": jsonResp("Delivery page", pageOf(ref("WebhookDelivery")))}}))
}

func registerCopilotMCP(p *paths) {
	p.add("GET", "/v1/copilot/config", op(opConfig{tag: "copilot", summary: "Get the copilot config",
		responses: M{"200": jsonResp("Config", ref("CopilotConfig"))}}))
	p.add("PATCH", "/v1/copilot/config", op(opConfig{tag: "copilot", summary: "Save the copilot config",
		reqBody: body(ref("SaveCopilotConfigRequest")), responses: M{"200": jsonResp("Saved", ref("CopilotConfig"))}}))
	p.add("POST", "/v1/copilot/suggest-reply", op(opConfig{tag: "copilot", summary: "Draft a reply (agentic; may propose write actions)",
		reqBody: body(ref("SuggestReplyRequest")), responses: M{"200": jsonResp("Result", ref("CopilotResult"))}}))
	p.add("POST", "/v1/copilot/summarize", op(opConfig{tag: "copilot", summary: "Summarize the conversation",
		reqBody: body(ref("SummarizeRequest")), responses: M{"200": jsonResp("Result", ref("CopilotResult"))}}))
	p.add("POST", "/v1/copilot/classify", op(opConfig{tag: "copilot", summary: "Classify into one of the given categories",
		reqBody: body(ref("ClassifyRequest")), responses: M{"200": jsonResp("Result", ref("CopilotResult"))}}))
	p.add("POST", "/v1/copilot/next-action", op(opConfig{tag: "copilot", summary: "Recommend the next action",
		reqBody: body(ref("NextActionRequest")), responses: M{"200": jsonResp("Result", ref("CopilotResult"))}}))

	// mcp servers
	p.add("GET", "/v1/mcp/servers", op(opConfig{tag: "mcp", summary: "List MCP servers", params: paginationParams(),
		responses: M{"200": jsonResp("Server page", pageOf(ref("McpServer")))}}))
	p.add("POST", "/v1/mcp/servers", op(opConfig{tag: "mcp", summary: "Register an MCP server",
		reqBody: body(ref("CreateMcpServerRequest")), responses: M{"201": jsonResp("Created", ref("McpServer"))}}))
	sidp := []M{pathParam("id", "server id")}
	p.add("GET", "/v1/mcp/servers/{id}", op(opConfig{tag: "mcp", summary: "Get an MCP server", params: sidp,
		responses: M{"200": jsonResp("Server", ref("McpServer")), "404": respRef("Error404")}}))
	p.add("PATCH", "/v1/mcp/servers/{id}", op(opConfig{tag: "mcp", summary: "Update an MCP server", params: sidp,
		reqBody: body(ref("UpdateMcpServerRequest")), responses: M{"200": jsonResp("Updated", ref("McpServer"))}}))
	p.add("DELETE", "/v1/mcp/servers/{id}", op(opConfig{tag: "mcp", summary: "Delete an MCP server", params: sidp,
		responses: M{"204": emptyResp("Deleted")}}))
	p.add("POST", "/v1/mcp/servers/{id}/test", op(opConfig{tag: "mcp", summary: "List the server's tools (connectivity test)", params: sidp,
		responses: M{"200": jsonResp("Tools", ref("McpToolList"))}}))

	// conversation-scoped tools + approvals
	cidp := []M{pathParam("id", "conversation id")}
	p.add("GET", "/v1/conversations/{id}/mcp/tools", op(opConfig{tag: "mcp", summary: "Tools available for a conversation",
		params: cidp, responses: M{"200": jsonResp("Tools", ref("McpToolList"))}}))
	p.add("POST", "/v1/conversations/{id}/mcp/run", op(opConfig{tag: "mcp", summary: "Run a tool (read executes; write creates a pending approval)",
		params: cidp, reqBody: body(ref("RunToolRequest")), responses: M{"200": jsonResp("Executed (read)", ref("McpRunResult")), "202": jsonResp("Pending approval (write)", ref("McpRunResult"))}}))
	p.add("GET", "/v1/conversations/{id}/copilot/tool-calls", op(opConfig{tag: "mcp",
		summary: "List a conversation's tool-call history (read) — copilot.use; 200 [] when none",
		params:  cidp, responses: M{"200": jsonResp("Tool-call logs", dataArr(ref("McpCallLog")))}}))
	p.add("GET", "/v1/conversations/{id}/copilot/approvals", op(opConfig{tag: "mcp",
		summary: "List a conversation's write-action approvals (read) — copilot.use; 200 [] when none",
		params:  cidp, responses: M{"200": jsonResp("Approvals", dataArr(ref("McpApproval")))}}))
	p.add("POST", "/v1/conversations/{id}/copilot/approvals/{approvalID}", op(opConfig{tag: "mcp", summary: "Approve/reject a proposed write action (approval triggers execution)",
		params: []M{pathParam("id", "conversation id"), pathParam("approvalID", "approval id")}, reqBody: body(ref("DecideRequest")),
		responses: M{"200": jsonResp("Decision applied", ref("McpRunResult"))}}))
}

func registerProductivity(p *paths) {
	p.crud("/v1/tags", "conversationtools", "tag", ref("Tag"), ref("CreateTagRequest"), ref("UpdateTagRequest"))
	p.crud("/v1/canned-responses", "conversationtools", "canned response", ref("CannedResponse"), ref("CreateCannedRequest"), ref("UpdateCannedRequest"))
	p.crud("/v1/close-reasons", "conversationtools", "close reason", ref("CloseReason"), ref("CreateCloseReasonRequest"), ref("UpdateCloseReasonRequest"))

	p.crud("/v1/sla/policies", "sla", "SLA policy", ref("SLAPolicy"), ref("CreateSLAPolicyRequest"), ref("UpdateSLAPolicyRequest"))
	p.add("GET", "/v1/sla/at-risk", op(opConfig{tag: "sla", summary: "Conversations approaching/breaching SLA",
		responses: M{"200": jsonResp("At-risk list", arr(ref("SLATracking")))}}))

	p.crud("/v1/csat/surveys", "csat", "CSAT survey", ref("Survey"), ref("CreateSurveyRequest"), ref("UpdateSurveyRequest"))
	p.add("GET", "/v1/csat/responses", op(opConfig{tag: "csat", summary: "List CSAT responses", params: paginationParams(),
		responses: M{"200": jsonResp("Response page", pageOf(ref("CSATResponse")))}}))
	p.add("POST", "/v1/csat/responses/{token}", op(opConfig{tag: "csat", summary: "Submit a CSAT answer (public token)",
		public: true, params: []M{pathParam("token", "survey response token")}, reqBody: body(ref("SubmitCSATRequest")),
		responses: M{"200": jsonResp("Recorded", ref("CSATResponse"))}}))

	// notifications
	p.add("GET", "/v1/notifications", op(opConfig{tag: "notifications", summary: "List notifications",
		params: append(paginationParams(), queryParam("unread", "Only unread when true")), responses: M{"200": jsonResp("Notification page", pageOf(ref("Notification")))}}))
	p.add("POST", "/v1/notifications/read-all", op(opConfig{tag: "notifications", summary: "Mark all notifications read",
		responses: M{"200": jsonResp("Result", ref("MarkAllReadResult"))}}))
	p.add("POST", "/v1/notifications/{id}/read", op(opConfig{tag: "notifications", summary: "Mark a notification read",
		params: []M{pathParam("id", "notification id")}, responses: M{"204": emptyResp("OK")}}))
	p.add("GET", "/v1/notifications/preferences", op(opConfig{tag: "notifications", summary: "Get notification preferences",
		responses: M{"200": jsonResp("Preferences", ref("NotificationPreferences"))}}))
	p.add("PATCH", "/v1/notifications/preferences", op(opConfig{tag: "notifications", summary: "Update notification preferences",
		reqBody: body(ref("UpdatePreferencesRequest")), responses: M{"200": jsonResp("Updated", ref("NotificationPreferences"))}}))
}

func registerInsights(p *paths) {
	q := func(extra ...M) []M {
		return append([]M{queryParam("from", "RFC3339 period start"), queryParam("to", "RFC3339 period end")}, extra...)
	}
	p.add("GET", "/v1/search/conversations", op(opConfig{tag: "search", summary: "Search conversations",
		params: append([]M{queryParam("q", "Search query")}, paginationParams()...), responses: M{"200": jsonResp("Hits", pageOf(ref("ConversationHit")))}}))
	p.add("GET", "/v1/search/contacts", op(opConfig{tag: "search", summary: "Search contacts",
		params: append([]M{queryParam("q", "Search query")}, paginationParams()...), responses: M{"200": jsonResp("Hits", pageOf(ref("ContactHit")))}}))
	p.add("GET", "/v1/search/messages", op(opConfig{tag: "search", summary: "Search messages",
		params: append([]M{queryParam("q", "Search query")}, paginationParams()...), responses: M{"200": jsonResp("Hits", pageOf(ref("MessageHit")))}}))

	p.add("GET", "/v1/reports/overview", op(opConfig{tag: "reports", summary: "Headline metrics", params: q(),
		responses: M{"200": jsonResp("Overview", ref("ReportOverview"))}}))
	p.add("GET", "/v1/reports/conversations", op(opConfig{tag: "reports", summary: "Conversation breakdowns", params: q(),
		responses: M{"200": jsonResp("Report", ref("ReportConversations"))}}))
	p.add("GET", "/v1/reports/agents", op(opConfig{tag: "reports", summary: "Per-agent productivity", params: q(),
		responses: M{"200": jsonResp("Report", ref("ReportAgents"))}}))
	p.add("GET", "/v1/reports/sectors", op(opConfig{tag: "reports", summary: "Per-sector volume", params: q(),
		responses: M{"200": jsonResp("Report", ref("ReportSectors"))}}))
	p.add("GET", "/v1/reports/automation", op(opConfig{tag: "reports", summary: "Automation summary", params: q(),
		responses: M{"200": jsonResp("Report", ref("ReportAutomation"))}}))
	p.add("GET", "/v1/reports/copilot", op(opConfig{tag: "reports", summary: "Copilot usage", params: q(),
		responses: M{"200": jsonResp("Report", ref("ReportCopilot"))}}))
	p.add("GET", "/v1/reports/sla", op(opConfig{tag: "reports", summary: "SLA outcomes", params: q(),
		responses: M{"200": jsonResp("Report", ref("ReportSLA"))}}))
	p.add("GET", "/v1/reports/csat", op(opConfig{tag: "reports", summary: "CSAT summary", params: q(),
		responses: M{"200": jsonResp("Report", ref("ReportCSAT"))}}))
	p.add("POST", "/v1/reports/export", op(opConfig{tag: "reports", summary: "Export a report (real file + signed URL)",
		params: q(queryParam("report", "Report name"), queryParam("format", "json|csv")), responses: M{"200": jsonResp("Export", ref("ReportExportResult"))}}))
	p.add("GET", "/v1/reports/downloads/{token}", op(opConfig{tag: "reports", summary: "Download an exported report (signed token)",
		public: true, params: []M{pathParam("token", "signed download token")},
		responses: M{"200": M{"description": "The report file", "content": M{"text/csv": M{"schema": M{"type": "string", "format": "binary"}}, "application/json": M{"schema": freeObject()}}}}}))
}

func registerPrivacyAttachments(p *paths) {
	p.add("POST", "/v1/privacy/contacts/{id}/export", op(opConfig{tag: "privacy", summary: "Request a contact data export (LGPD)",
		params: []M{pathParam("id", "contact id")}, responses: M{"202": jsonResp("Pending export", ref("PrivacyExport"))}}))
	p.add("POST", "/v1/privacy/contacts/{id}/anonymize", op(opConfig{tag: "privacy", summary: "Anonymize a contact (LGPD)",
		params: []M{pathParam("id", "contact id")}, responses: M{"200": jsonResp("Result", freeObject())}}))
	p.add("GET", "/v1/privacy/exports/{id}", op(opConfig{tag: "privacy", summary: "Get an export request",
		params: []M{pathParam("id", "export id")}, responses: M{"200": jsonResp("Export", ref("PrivacyExport"))}}))
	p.add("GET", "/v1/privacy/retention", op(opConfig{tag: "privacy", summary: "Get retention settings",
		responses: M{"200": jsonResp("Retention", ref("RetentionPolicy"))}}))
	p.add("PATCH", "/v1/privacy/retention", op(opConfig{tag: "privacy", summary: "Update retention settings",
		reqBody: body(ref("UpdateRetentionRequest")), responses: M{"200": jsonResp("Updated", ref("RetentionPolicy"))}}))
	p.add("GET", "/v1/privacy/downloads/{token}", op(opConfig{tag: "privacy", summary: "Download an export bundle (signed token)",
		public: true, params: []M{pathParam("token", "signed download token")},
		responses: M{"200": M{"description": "The export bundle", "content": M{"application/json": M{"schema": freeObject()}}}}}))
	p.add("GET", "/v1/audit", op(opConfig{tag: "audit", summary: "List audit log entries",
		params: append(paginationParams(), queryParam("action", "Filter by action"), queryParam("actor_id", "Filter by actor")), responses: M{"200": jsonResp("Audit page", pageOf(ref("AuditLog")))}}))

	p.add("POST", "/v1/attachments/upload-url", op(opConfig{tag: "attachments", summary: "Create a signed upload URL",
		reqBody: body(ref("CreateUploadURLRequest")), responses: M{"200": jsonResp("Upload target", ref("UploadURLResponse"))}}))
	p.add("POST", "/v1/attachments/confirm", op(opConfig{tag: "attachments", summary: "Confirm an uploaded attachment",
		reqBody: body(ref("ConfirmAttachmentRequest")), responses: M{"200": jsonResp("Attachment", ref("AttachmentRecord"))}}))
	p.add("GET", "/v1/attachments/{id}", op(opConfig{tag: "attachments", summary: "Get attachment metadata",
		params: []M{pathParam("id", "attachment id")}, responses: M{"200": jsonResp("Attachment", ref("AttachmentRecord"))}}))
	p.add("GET", "/v1/attachments/{id}/download", op(opConfig{tag: "attachments", summary: "Download (or redirect to) an attachment",
		params: []M{pathParam("id", "attachment id")}, responses: M{"200": M{"description": "The file"}, "302": emptyResp("Redirect to storage")}}))
	p.add("PUT", "/v1/attachments/blobs/{token}", op(opConfig{tag: "attachments", summary: "Upload bytes to the local signed sink (local provider)",
		public: true, params: []M{pathParam("token", "signed upload token")},
		reqBody:   M{"required": true, "content": M{"application/octet-stream": M{"schema": M{"type": "string", "format": "binary"}}}},
		responses: M{"200": emptyResp("Stored")}}))
	// Integration rail: public, JWT-less, signed media download (the data_url in
	// the outbound ChannelOutboundMessage points here). Token is the only credential.
	p.add("GET", "/v1/channel-media/{token}", op(opConfig{tag: "channels",
		summary: "Download outbound media via a signed token (integration rail; no JWT)",
		public:  true, params: []M{pathParam("token", "signed, expiring media token")},
		responses: M{"200": M{"description": "The file"}, "302": emptyResp("Redirect to storage")}}))
}

// Package openapi builds the OpenAPI 3.1 description of the public HTTP API
// (every /v1 route, the standard error envelope, cursor pagination and Bearer
// JWT auth) and serves it at GET /openapi.json. The document is the single
// source of truth: the same value is marshaled to JSON (served) and to YAML
// (the committed docs/openapi.yaml copy), so the contract a typed frontend
// client generates from never drifts from the code.
package openapi

import "strings"

// M is a JSON/YAML object literal.
type M = map[string]any

// ── schema helpers ────────────────────────────────────────────────────────────

func ref(name string) M { return M{"$ref": "#/components/schemas/" + name} }
func str() M            { return M{"type": "string"} }
func boolean() M        { return M{"type": "boolean"} }
func integer() M        { return M{"type": "integer"} }
func number() M         { return M{"type": "number"} }
func dateTime() M       { return M{"type": "string", "format": "date-time"} }
func arr(items M) M     { return M{"type": "array", "items": items} }

// tagIDArray documents the conversation/contact `tags` field, which always
// stores canonical tag IDs (never names). Requests may send IDs or names; the
// server resolves names to IDs, so what is stored and returned is always IDs.
func tagIDArray() M {
	return M{"type": "array", "items": str(),
		"description": "Tag IDs (never names). On write you may send a tag ID or a tag name; the server resolves names to canonical IDs, so values are always stored and returned as IDs."}
}
func freeObject() M { return M{"type": "object", "additionalProperties": true} }
func stringMap() M  { return M{"type": "object", "additionalProperties": M{"type": "string"}} }
func boolMap() M    { return M{"type": "object", "additionalProperties": M{"type": "boolean"}} }
func enum(vals ...string) M {
	return M{"type": "string", "enum": anySlice(vals)}
}

// describedStr is a string schema with a human description, used to document
// validation/normalization rules a typed client should know about.
func describedStr(desc string) M { return M{"type": "string", "description": desc} }

// contactIdentityChannelEnum is the closed set of channels a contact external
// identity may use (domain/contacts/entity.SupportedIdentityChannels): the real
// channel-connection types plus the CRM-only identity channels (sms/email/crm).
func contactIdentityChannelEnum() M {
	return enum("whatsapp", "telegram", "instagram", "webchat", "sms", "email", "api", "crm", "custom")
}

// conversationStatusEnum is the single source of truth for the conversation
// status vocabulary (domain/conversations/entity.Status). The same set is used by
// the Conversation schema, the PATCH /conversations/{id} body and the
// GET /conversations ?status= filter, so a client can move a conversation to
// exactly the values the list filter understands.
func conversationStatusEnum() M {
	return enum("new", "automation", "queued", "assigned", "waiting_customer",
		"waiting_agent", "transferred", "resolved", "closed", "archived")
}

// webhookEventEnum is the closed set of outbound-webhook wire events
// (domain/webhooks/entity.SupportedEvents), following Chatwoot's underscore
// convention. Names Chatwoot also has are identical (conversation_created,
// conversation_status_changed, message_created).
func webhookEventEnum() M {
	return enum("conversation_created", "conversation_status_changed",
		"conversation_assigned", "conversation_transferred", "message_created",
		"sla_breached", "automation_completed", "automation_failed")
}

func object(props M, required ...string) M {
	o := M{"type": "object", "properties": props}
	if len(required) > 0 {
		o["required"] = anySlice(required)
	}
	return o
}

// pageOf wraps an item schema in the cursor-pagination envelope { data, page }.
func pageOf(item M) M {
	return object(M{"data": arr(item), "page": ref("PageInfo")})
}

// dataArr is a non-paginated { data: [item] } envelope.
func dataArr(item M) M {
	return object(M{"data": arr(item)})
}

func anySlice(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

// ── request/response/param helpers ───────────────────────────────────────────

func body(schema M) M {
	return M{"required": true, "content": M{"application/json": M{"schema": schema}}}
}

func jsonResp(desc string, schema M) M {
	return M{"description": desc, "content": M{"application/json": M{"schema": schema}}}
}

func emptyResp(desc string) M { return M{"description": desc} }

func pathParam(name, desc string) M {
	return M{"name": name, "in": "path", "required": true, "description": desc, "schema": str()}
}

func queryParam(name, desc string) M {
	return M{"name": name, "in": "query", "required": false, "description": desc, "schema": str()}
}

// headerParam describes an optional request header parameter.
func headerParam(name, desc string) M {
	return M{"name": name, "in": "header", "required": false, "description": desc, "schema": str()}
}

// paginationParams are the shared keyset (cursor) pagination query params.
func paginationParams() []M {
	return []M{
		{"name": "limit", "in": "query", "required": false, "description": "Page size (max 100).", "schema": M{"type": "integer", "minimum": 1, "maximum": 100}},
		{"name": "cursor", "in": "query", "required": false, "description": "Opaque keyset cursor from a previous page's next_cursor.", "schema": str()},
	}
}

// respRef references a reusable response from components/responses.
func respRef(name string) M { return M{"$ref": "#/components/responses/" + name} }

// errorResponse references the standard error envelope (used for per-operation
// responses with a custom description, e.g. a specific 404).
func errorResponse(desc string) M {
	return M{"description": desc, "content": M{"application/json": M{"schema": ref("Error")}}}
}

// errorResponses are the reusable error envelope responses shared by every
// operation, so the standard codes are declared once.
func errorResponses() M {
	mk := func(desc string) M {
		return M{"description": desc, "content": M{"application/json": M{"schema": ref("Error")}}}
	}
	return M{
		"Error400": mk("Validation error (validation_error)."),
		"Error401": mk("Missing/invalid access token (unauthorized)."),
		"Error403": mk("Insufficient permission (forbidden)."),
		"Error404": mk("Resource not found (not_found)."),
		"Error409": mk("Conflict (conflict)."),
		"Error429": mk("Rate limited (rate_limited)."),
		"Error500": mk("Internal error (internal_error)."),
	}
}

// ── operation builder ─────────────────────────────────────────────────────────

type opConfig struct {
	tag       string
	summary   string
	public    bool
	params    []M
	reqBody   M
	responses M // success responses, keyed by status code string
}

func op(c opConfig) M {
	resp := M{}
	for k, v := range c.responses {
		resp[k] = v
	}
	// Standard error envelope responses, common to every operation (declared once
	// in components/responses).
	resp["400"] = respRef("Error400")
	resp["429"] = respRef("Error429")
	resp["500"] = respRef("Error500")
	if !c.public {
		resp["401"] = respRef("Error401")
		resp["403"] = respRef("Error403")
	}
	o := M{"tags": []any{c.tag}, "summary": c.summary, "responses": resp}
	if c.reqBody != nil {
		o["requestBody"] = c.reqBody
	}
	if len(c.params) > 0 {
		o["parameters"] = mSlice(c.params)
	}
	if c.public {
		o["security"] = []any{} // override the global Bearer requirement
	}
	return o
}

func mSlice(ms []M) []any {
	out := make([]any, len(ms))
	for i, m := range ms {
		out[i] = m
	}
	return out
}

// paths accumulates operations, merging multiple methods on the same path.
type paths struct{ m M }

func newPaths() *paths { return &paths{m: M{}} }

func (p *paths) add(method, path string, o M) {
	if _, ok := o["operationId"]; !ok {
		o["operationId"] = operationID(method, path)
	}
	entry, ok := p.m[path].(M)
	if !ok {
		entry = M{}
		p.m[path] = entry
	}
	entry[strings.ToLower(method)] = o
}

// operationID derives a stable, unique camelCase id from the method and path so
// generated clients get readable method names (e.g. getConversationsByIdMessages).
func operationID(method, path string) string {
	var b strings.Builder
	b.WriteString(strings.ToLower(method))
	for _, seg := range strings.Split(strings.TrimPrefix(path, "/v1/"), "/") {
		if seg == "" {
			continue
		}
		if strings.HasPrefix(seg, "{") {
			b.WriteString("By")
			b.WriteString(pascal(strings.Trim(seg, "{}")))
			continue
		}
		b.WriteString(pascal(seg))
	}
	return b.String()
}

func pascal(s string) string {
	var b strings.Builder
	for _, f := range strings.FieldsFunc(s, func(r rune) bool { return r == '-' || r == '_' }) {
		b.WriteString(strings.ToUpper(f[:1]))
		b.WriteString(f[1:])
	}
	return b.String()
}

// crud registers a standard list/create/get/update/delete group under base.
func (p *paths) crud(base, tag, name string, item, create, update M) {
	p.add("GET", base, op(opConfig{tag: tag, summary: "List " + name, params: paginationParams(),
		responses: M{"200": jsonResp(name+" page", pageOf(item))}}))
	p.add("POST", base, op(opConfig{tag: tag, summary: "Create " + name, reqBody: body(create),
		responses: M{"201": jsonResp("Created", item), "409": respRef("Error409")}}))
	idp := []M{pathParam("id", name+" id")}
	p.add("GET", base+"/{id}", op(opConfig{tag: tag, summary: "Get " + name, params: idp,
		responses: M{"200": jsonResp(name, item), "404": errorResponse("Not found.")}}))
	p.add("PATCH", base+"/{id}", op(opConfig{tag: tag, summary: "Update " + name, params: idp, reqBody: body(update),
		responses: M{"200": jsonResp("Updated", item), "404": errorResponse("Not found.")}}))
	p.add("DELETE", base+"/{id}", op(opConfig{tag: tag, summary: "Delete " + name, params: idp,
		responses: M{"204": emptyResp("Deleted"), "404": errorResponse("Not found.")}}))
}

// ── document ──────────────────────────────────────────────────────────────────

// Build returns the full OpenAPI 3.1 document as a plain map (deterministic when
// marshaled to JSON or YAML).
func Build() M {
	p := newPaths()
	registerAuth(p)
	registerTenantIAM(p)
	registerOrg(p)
	registerConversations(p)
	registerChannels(p)
	registerIntegrations(p)
	registerCopilotMCP(p)
	registerProductivity(p)
	registerInsights(p)
	registerPrivacyAttachments(p)

	return M{
		"openapi": "3.1.0",
		"info": M{
			"title":       "chat-smsnet-omnichannel API",
			"version":     "1.0.0",
			"description": "Multi-tenant omnichannel chat backend. All endpoints are under /v1. Errors use the standard envelope { error: { code, message, details?, request_id? } }; lists use keyset (cursor) pagination { data, page: { next_cursor?, has_more } }; protected endpoints require a Bearer JWT access token (the tenant is derived from the token, never a header).",
		},
		"servers":  []any{M{"url": "/", "description": "Same origin"}},
		"security": []any{M{"bearerAuth": []any{}}},
		"tags":     tags(),
		"components": M{
			"securitySchemes": M{
				"bearerAuth": M{"type": "http", "scheme": "bearer", "bearerFormat": "JWT"},
			},
			"responses": errorResponses(),
			"schemas":   schemas(),
		},
		"paths": p.m,
	}
}

func tags() []any {
	desc := []struct{ name, description string }{
		{"auth", "Login, token rotation, signup/verification, password reset, self profile."},
		{"tenant", "The current tenant."},
		{"iam", "Users, roles and permissions."},
		{"sectors", "Service sectors (departments)."},
		{"queues", "Distribution queues."},
		{"presence", "Agent presence and load."},
		{"conversations", "Conversations, messages, notes, lifecycle and the events timeline."},
		{"contacts", "Read contacts (CRM-style profile)."},
		{"routing", "Assignment, transfer and queue distribution."},
		{"channels", "Channel connections and inbound ingestion."},
		{"automation", "External automation integrations and runs."},
		{"providerhub", "smsnet-integrations config and on-demand external queries."},
		{"webhooks", "Outbound webhook subscriptions and deliveries."},
		{"copilot", "AI copilot configuration and inference (agentic tool loop)."},
		{"mcp", "MCP servers, tool discovery, manual runs and write-action approvals."},
		{"conversationtools", "Tags, canned responses and close reasons."},
		{"businesshours", "Holidays and sector business status."},
		{"sla", "SLA policies and tracking."},
		{"notifications", "Operator notifications and preferences."},
		{"csat", "Satisfaction surveys and responses."},
		{"search", "Full-text search over conversations, contacts and messages."},
		{"reports", "Operational reports and exports."},
		{"privacy", "LGPD export, anonymization and retention."},
		{"audit", "Audit log."},
		{"attachments", "Signed upload/download of message attachments."},
	}
	out := make([]any, len(desc))
	for i, d := range desc {
		out[i] = M{"name": d.name, "description": d.description}
	}
	return out
}

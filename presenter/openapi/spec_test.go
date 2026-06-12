package openapi

import (
	"encoding/json"
	"strings"
	"testing"
)

// collectRefs walks the document collecting every "$ref" string.
func collectRefs(v any, out *[]string) {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if k == "$ref" {
				if s, ok := val.(string); ok {
					*out = append(*out, s)
				}
				continue
			}
			collectRefs(val, out)
		}
	case []any:
		for _, e := range t {
			collectRefs(e, out)
		}
	}
}

func TestSpec_StructurallyValid(t *testing.T) {
	doc := Build()

	if doc["openapi"] != "3.1.0" {
		t.Fatalf("openapi version = %v, want 3.1.0", doc["openapi"])
	}
	info, _ := doc["info"].(M)
	if info["title"] == "" || info["version"] == "" {
		t.Fatalf("info.title/version missing: %v", info)
	}
	// Global Bearer security + scheme.
	comps, _ := doc["components"].(M)
	schemes, _ := comps["securitySchemes"].(M)
	if _, ok := schemes["bearerAuth"]; !ok {
		t.Fatal("missing bearerAuth security scheme")
	}

	paths, _ := doc["paths"].(M)
	if len(paths) < 80 {
		t.Fatalf("expected the full /v1 surface, got only %d paths", len(paths))
	}

	// Every operation: under /v1 or a documented public path, has >=1 2xx response.
	ops := 0
	for path, item := range paths {
		methods, _ := item.(M)
		for method, raw := range methods {
			o, _ := raw.(M)
			resp, _ := o["responses"].(M)
			if len(resp) == 0 {
				t.Errorf("%s %s has no responses", method, path)
				continue
			}
			has2xx := false
			for code := range resp {
				if strings.HasPrefix(code, "2") {
					has2xx = true
				}
			}
			if !has2xx {
				t.Errorf("%s %s has no 2xx response", method, path)
			}
			if _, ok := o["tags"]; !ok {
				t.Errorf("%s %s has no tag", method, path)
			}
			ops++
		}
	}
	if ops < 120 {
		t.Fatalf("expected ~130 operations, got %d", ops)
	}
}

func TestSpec_AllRefsResolve(t *testing.T) {
	doc := Build()
	comps, _ := doc["components"].(M)
	schemas, _ := comps["schemas"].(M)
	responses, _ := comps["responses"].(M)

	var refs []string
	collectRefs(doc, &refs)
	if len(refs) == 0 {
		t.Fatal("no $refs found — schemas not wired")
	}
	for _, r := range refs {
		switch {
		case strings.HasPrefix(r, "#/components/schemas/"):
			name := strings.TrimPrefix(r, "#/components/schemas/")
			if _, ok := schemas[name]; !ok {
				t.Errorf("dangling schema ref: %s", r)
			}
		case strings.HasPrefix(r, "#/components/responses/"):
			name := strings.TrimPrefix(r, "#/components/responses/")
			if _, ok := responses[name]; !ok {
				t.Errorf("dangling response ref: %s", r)
			}
		default:
			t.Errorf("unexpected ref form: %s", r)
		}
	}
}

func TestSpec_MarshalsToValidJSON(t *testing.T) {
	var v any
	if err := json.Unmarshal(JSON(), &v); err != nil {
		t.Fatalf("served JSON is not valid JSON: %v", err)
	}
}

// TestSpec_CoversEveryDomain asserts a representative (and the tricky) path from
// each /v1 domain is present, so the contract stays complete.
func TestSpec_CoversEveryDomain(t *testing.T) {
	doc := Build()
	paths, _ := doc["paths"].(M)
	want := []struct{ path, method string }{
		{"/v1/auth/login", "post"}, {"/v1/me", "get"}, {"/v1/me/change-password", "post"},
		{"/v1/tenants/current", "get"},
		{"/v1/users", "get"}, {"/v1/users/{id}", "patch"}, {"/v1/users/invite", "post"}, {"/v1/roles", "post"},
		{"/v1/sectors", "get"}, {"/v1/queues", "post"},
		{"/v1/agents/presence", "get"}, {"/v1/agents/presence/status", "post"},
		{"/v1/conversations", "get"}, {"/v1/conversations/{id}/messages", "post"},
		{"/v1/conversations/{id}/events", "get"}, {"/v1/contacts/{id}", "get"},
		{"/v1/conversations/{id}/messages/{mid}", "delete"}, {"/v1/conversations/{id}/close", "post"},
		{"/v1/conversations/{id}/assign", "post"}, {"/v1/conversations/{id}/transfer", "post"},
		{"/v1/conversations/{id}/tags", "post"}, {"/v1/conversations/{id}/sla", "get"},
		{"/v1/conversations/{id}/external/cliente", "get"}, {"/v1/conversations/{id}/external/liberacao", "post"},
		{"/v1/conversations/{id}/mcp/tools", "get"}, {"/v1/conversations/{id}/mcp/run", "post"},
		{"/v1/conversations/{id}/copilot/approvals/{approvalID}", "post"},
		{"/v1/channels", "post"}, {"/v1/channels/{id}/test", "post"},
		{"/v1/inbound/channel/{channel}/messages", "post"},
		{"/v1/automation/integrations", "get"}, {"/v1/automation/runs/{id}", "get"}, {"/v1/automation/callbacks/{tenant_id}", "post"},
		{"/v1/providerhub/config", "get"}, {"/v1/providerhub/config/test", "post"},
		{"/v1/webhooks", "get"}, {"/v1/webhooks/{id}/deliveries", "get"},
		{"/v1/copilot/config", "get"}, {"/v1/copilot/suggest-reply", "post"},
		{"/v1/mcp/servers", "post"}, {"/v1/mcp/servers/{id}/test", "post"},
		{"/v1/tags", "get"}, {"/v1/canned-responses", "post"}, {"/v1/close-reasons/{id}", "delete"},
		{"/v1/holidays", "get"}, {"/v1/sectors/{id}/business-status", "get"},
		{"/v1/sla/policies", "post"}, {"/v1/sla/at-risk", "get"},
		{"/v1/notifications", "get"}, {"/v1/notifications/read-all", "post"}, {"/v1/notifications/preferences", "patch"},
		{"/v1/csat/surveys", "post"}, {"/v1/csat/responses", "get"}, {"/v1/csat/responses/{token}", "post"},
		{"/v1/search/conversations", "get"}, {"/v1/search/messages", "get"},
		{"/v1/reports/overview", "get"}, {"/v1/reports/export", "post"}, {"/v1/reports/downloads/{token}", "get"},
		{"/v1/privacy/contacts/{id}/export", "post"}, {"/v1/privacy/retention", "patch"},
		{"/v1/privacy/downloads/{token}", "get"}, {"/v1/audit", "get"},
		{"/v1/attachments/upload-url", "post"}, {"/v1/attachments/{id}/download", "get"},
		{"/v1/attachments/blobs/{token}", "put"},
		{"/v1/routing/run", "post"},
	}
	for _, w := range want {
		item, ok := paths[w.path].(M)
		if !ok {
			t.Errorf("missing path %s", w.path)
			continue
		}
		if _, ok := item[w.method]; !ok {
			t.Errorf("missing %s %s", strings.ToUpper(w.method), w.path)
		}
	}
}

// TestSpec_ContractAdditions verifies this change's new shapes are typed.
func TestSpec_ContractAdditions(t *testing.T) {
	doc := Build()
	comps, _ := doc["components"].(M)
	schemas, _ := comps["schemas"].(M)

	// Contact + Customer-360 are typed (no longer additionalProperties:true).
	for _, name := range []string{"Contact", "ContactExternalID", "Cliente", "Fatura", "Plano", "Empresa", "ClienteResult", "ConversationEvent"} {
		if _, ok := schemas[name]; !ok {
			t.Errorf("missing schema %q", name)
		}
	}
	// Cliente carries faturas[]; ClienteResult is a oneOf (contract selector).
	cliente, _ := schemas["Cliente"].(M)
	props, _ := cliente["properties"].(M)
	if _, ok := props["faturas"]; !ok {
		t.Error("Cliente must include faturas[]")
	}
	if _, ok := schemas["ClienteResult"].(M)["oneOf"]; !ok {
		t.Error("ClienteResult must be modeled as a oneOf")
	}
	// Conversation exposes unread_count + last_read_at.
	conv, _ := schemas["Conversation"].(M)
	cprops, _ := conv["properties"].(M)
	if _, ok := cprops["unread_count"]; !ok {
		t.Error("Conversation must expose unread_count")
	}
	if _, ok := cprops["last_read_at"]; !ok {
		t.Error("Conversation must expose last_read_at")
	}
}

// TestSpec_PublicEndpointsHaveNoAuth verifies the auth-free endpoints opt out of
// the global Bearer requirement (security: []).
func TestSpec_PublicEndpointsHaveNoAuth(t *testing.T) {
	doc := Build()
	paths, _ := doc["paths"].(M)
	public := []struct{ path, method string }{
		{"/v1/auth/login", "post"}, {"/v1/auth/signup", "post"},
		{"/v1/csat/responses/{token}", "post"}, {"/v1/reports/downloads/{token}", "get"},
		{"/v1/inbound/channel/{channel}/messages", "post"},
	}
	for _, w := range public {
		o, _ := paths[w.path].(M)[w.method].(M)
		sec, ok := o["security"]
		if !ok {
			t.Errorf("%s %s should declare security: [] (public)", w.method, w.path)
			continue
		}
		if arr, _ := sec.([]any); len(arr) != 0 {
			t.Errorf("%s %s should have empty security, got %v", w.method, w.path, sec)
		}
	}
}

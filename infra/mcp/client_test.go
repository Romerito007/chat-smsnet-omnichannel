package mcp

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/mcp/entity"
)

// mcpServer is a minimal JSON-RPC MCP server over HTTP for tests. It answers
// initialize, tools/list and tools/call, and records the auth header it saw.
func mcpServer(t *testing.T, tools string) (*httptest.Server, *string) {
	t.Helper()
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		var req struct {
			ID     any    `json:"id"`
			Method string `json:"method"`
			Params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		_ = json.Unmarshal(body, &req)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Mcp-Session-Id", "sess-123")
		var result string
		switch req.Method {
		case "initialize":
			result = `{"protocolVersion":"2024-11-05","capabilities":{},"serverInfo":{"name":"test"}}`
		case "tools/list":
			result = `{"tools":` + tools + `}`
		case "tools/call":
			result = `{"content":[{"type":"text","text":"called ` + req.Params.Name + `"}],"isError":false}`
		default:
			result = `{}`
		}
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":`+result+`}`)
	}))
	t.Cleanup(srv.Close)
	return srv, &gotAuth
}

func TestClient_ListTools_DiscoversDynamically(t *testing.T) {
	tools := `[{"name":"smsnet_consultar_cliente","description":"lookup","inputSchema":{"type":"object"}},
	           {"name":"smsnet_listar_planos","description":"plans","inputSchema":{"type":"object"}}]`
	srv, gotAuth := mcpServer(t, tools)
	conn := &entity.ServerConnection{BaseURL: srv.URL, AuthHeader: "Authorization", AuthToken: "Bearer secret"}

	specs, err := NewClient().ListTools(t.Context(), conn)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(specs) != 2 || specs[0].Name != "smsnet_consultar_cliente" || specs[1].Name != "smsnet_listar_planos" {
		t.Fatalf("tools not discovered dynamically: %+v", specs)
	}
	if *gotAuth != "Bearer secret" {
		t.Errorf("auth header not forwarded: %q", *gotAuth)
	}
}

func TestClient_CallTool(t *testing.T) {
	srv, _ := mcpServer(t, `[{"name":"smsnet_consultar_cliente"}]`)
	conn := &entity.ServerConnection{BaseURL: srv.URL}
	res, err := NewClient().CallTool(t.Context(), conn, "smsnet_consultar_cliente", map[string]any{"cpf": "123"})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.IsError || res.Text != "called smsnet_consultar_cliente" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestClient_SSEResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"tools\":[{\"name\":\"t1\"}]}}\n\n")
	}))
	t.Cleanup(srv.Close)
	conn := &entity.ServerConnection{BaseURL: srv.URL}
	specs, err := NewClient().ListTools(t.Context(), conn)
	if err != nil {
		t.Fatalf("list over SSE: %v", err)
	}
	if len(specs) != 1 || specs[0].Name != "t1" {
		t.Fatalf("SSE parse failed: %+v", specs)
	}
}

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

// mcpServerAtMCPPath emulates mark3labs' NewStreamableHTTPServer: it serves MCP
// ONLY at /mcp and 404s every other path (notably the root "/"). It records the
// path the client actually hit.
func mcpServerAtMCPPath(t *testing.T, tools string) (*httptest.Server, *string) {
	t.Helper()
	var gotPath string
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Method string `json:"method"`
		}
		_ = json.Unmarshal(body, &req)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Mcp-Session-Id", "sess-1")
		var result string
		switch req.Method {
		case "initialize":
			result = `{"protocolVersion":"2024-11-05","capabilities":{}}`
		case "tools/list":
			result = `{"tools":` + tools + `}`
		default:
			result = `{}`
		}
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":`+result+`}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &gotPath
}

// TestClient_ListTools_RoutesBarePathToMCP is the regression for the 502
// "could not list tools": a base_url WITHOUT a path (e.g. http://host:8086) must be
// routed to .../mcp (the Streamable HTTP convention), not the root — which a real
// mark3labs server 404s.
func TestClient_ListTools_RoutesBarePathToMCP(t *testing.T) {
	srv, gotPath := mcpServerAtMCPPath(t, `[{"name":"consultar_cliente","description":"lookup"}]`)
	conn := &entity.ServerConnection{BaseURL: srv.URL} // no path — mark3labs serves at /mcp

	specs, err := NewClient().ListTools(t.Context(), conn)
	if err != nil {
		t.Fatalf("tools/list against a /mcp-only server must succeed, got: %v", err)
	}
	if len(specs) != 1 || specs[0].Name != "consultar_cliente" {
		t.Fatalf("tools not discovered: %+v", specs)
	}
	if *gotPath != "/mcp" {
		t.Errorf("client must hit /mcp for a bare base_url, hit %q", *gotPath)
	}
}

func TestEndpointURL(t *testing.T) {
	cases := map[string]string{
		"http://127.0.0.1:8086":      "http://127.0.0.1:8086/mcp", // bare host → /mcp
		"http://127.0.0.1:8086/":     "http://127.0.0.1:8086/mcp", // root → /mcp
		"http://127.0.0.1:8086/mcp":  "http://127.0.0.1:8086/mcp", // explicit → unchanged
		"http://host/custom/rpc":     "http://host/custom/rpc",    // custom path → unchanged
		"https://gw.example.com/mcp": "https://gw.example.com/mcp",
	}
	for in, want := range cases {
		if got := endpointURL(in); got != want {
			t.Errorf("endpointURL(%q) = %q, want %q", in, got, want)
		}
	}
}

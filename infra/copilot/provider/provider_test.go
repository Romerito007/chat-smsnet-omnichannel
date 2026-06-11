package provider

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
)

func samplePC() contracts.PromptContext {
	return contracts.PromptContext{
		Channel:    "whatsapp",
		Transcript: []contracts.Turn{{Role: "customer", Text: "my internet is down"}},
	}
}

func suggestReq(baseURL string) contracts.Request {
	return contracts.Request{
		Action: entity.ActionSuggestReply, Model: "test-model", APIKey: "sk-test",
		BaseURL: baseURL, Temperature: 0.4, MaxTokens: 128, Context: samplePC(),
	}
}

// captureServer records the last request (path, headers, body) and returns the
// supplied JSON, simulating each provider's API without calling the real one.
func captureServer(t *testing.T, respJSON string) (*httptest.Server, *capturedReq) {
	t.Helper()
	cap := &capturedReq{header: http.Header{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		cap.path = r.URL.Path
		cap.query = r.URL.RawQuery
		cap.header = r.Header.Clone()
		_ = json.Unmarshal(body, &cap.body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(respJSON))
	}))
	t.Cleanup(srv.Close)
	return srv, cap
}

type capturedReq struct {
	path   string
	query  string
	header http.Header
	body   map[string]any
}

// ── OpenAI-compatible family (openai, mistral, deepseek, perplexity) ───────────

func TestOpenAICompatible_SuggestReply(t *testing.T) {
	resp := `{"choices":[{"message":{"content":"Hi! We are looking into it."}}],"usage":{"prompt_tokens":12,"completion_tokens":7}}`
	for _, p := range []struct {
		name string
		make func() contracts.AIProvider
	}{
		{"openai", NewOpenAI},
		{"mistral", NewMistral},
		{"deepseek", NewDeepSeek},
		{"perplexity", NewPerplexity},
	} {
		t.Run(p.name, func(t *testing.T) {
			srv, cap := captureServer(t, resp)
			got, err := p.make().Infer(t.Context(), suggestReq(srv.URL))
			if err != nil {
				t.Fatalf("infer: %v", err)
			}
			if got.Text != "Hi! We are looking into it." || got.TokensInput != 12 || got.TokensOutput != 7 {
				t.Fatalf("unexpected response: %+v", got)
			}
			if !strings.HasSuffix(cap.path, "/chat/completions") {
				t.Errorf("unexpected path %q", cap.path)
			}
			if cap.header.Get("Authorization") != "Bearer sk-test" {
				t.Errorf("missing bearer auth: %q", cap.header.Get("Authorization"))
			}
			if cap.body["model"] != "test-model" {
				t.Errorf("model not sent: %v", cap.body["model"])
			}
		})
	}
}

func TestOpenAICompatible_Classify(t *testing.T) {
	srv, _ := captureServer(t, `{"choices":[{"message":{"content":"billing"}}],"usage":{"prompt_tokens":5,"completion_tokens":1}}`)
	req := suggestReq(srv.URL)
	req.Action = entity.ActionClassify
	req.Context.Instruction = "categories: billing, technical, sales"
	got, err := NewOpenAI().Infer(t.Context(), req)
	if err != nil {
		t.Fatalf("infer: %v", err)
	}
	if len(got.Categories) != 1 || got.Categories[0] != "billing" {
		t.Fatalf("classify categories = %v, want [billing]", got.Categories)
	}
}

func TestOpenAICompatible_ForwardsToolsAndSurfacesToolCalls(t *testing.T) {
	resp := `{"choices":[{"message":{"content":"","tool_calls":[{"id":"call_1","function":{"name":"get_invoice","arguments":"{\"id\":\"42\"}"}}]}}],"usage":{"prompt_tokens":3,"completion_tokens":2}}`
	srv, cap := captureServer(t, resp)
	req := suggestReq(srv.URL)
	req.Tools = []contracts.ToolDefinition{{Name: "get_invoice", Description: "read invoice", Schema: map[string]any{"type": "object"}, ReadOnly: true}}
	got, err := NewOpenAI().Infer(t.Context(), req)
	if err != nil {
		t.Fatalf("infer: %v", err)
	}
	if cap.body["tools"] == nil {
		t.Error("tools were not forwarded to the provider")
	}
	if len(got.ToolCalls) != 1 || got.ToolCalls[0].Name != "get_invoice" || got.ToolCalls[0].ID != "call_1" {
		t.Fatalf("tool call not surfaced: %+v", got.ToolCalls)
	}
}

// ── Anthropic ──────────────────────────────────────────────────────────────────

func TestAnthropic_SuggestReply(t *testing.T) {
	resp := `{"content":[{"type":"text","text":"Hello from Claude"}],"usage":{"input_tokens":9,"output_tokens":4}}`
	srv, cap := captureServer(t, resp)
	got, err := NewAnthropic().Infer(t.Context(), suggestReq(srv.URL))
	if err != nil {
		t.Fatalf("infer: %v", err)
	}
	if got.Text != "Hello from Claude" || got.TokensInput != 9 || got.TokensOutput != 4 {
		t.Fatalf("unexpected response: %+v", got)
	}
	if !strings.HasSuffix(cap.path, "/v1/messages") {
		t.Errorf("unexpected path %q", cap.path)
	}
	if cap.header.Get("x-api-key") != "sk-test" || cap.header.Get("anthropic-version") == "" {
		t.Errorf("missing anthropic auth/version headers: %v", cap.header)
	}
}

func TestAnthropic_SurfacesToolUse(t *testing.T) {
	resp := `{"content":[{"type":"tool_use","id":"tu_1","name":"open_ticket","input":{"reason":"offline"}}],"usage":{"input_tokens":2,"output_tokens":3}}`
	srv, _ := captureServer(t, resp)
	req := suggestReq(srv.URL)
	req.Tools = []contracts.ToolDefinition{{Name: "open_ticket", Description: "write", Schema: map[string]any{"type": "object"}}}
	got, err := NewAnthropic().Infer(t.Context(), req)
	if err != nil {
		t.Fatalf("infer: %v", err)
	}
	if len(got.ToolCalls) != 1 || got.ToolCalls[0].Name != "open_ticket" || !strings.Contains(got.ToolCalls[0].Arguments, "offline") {
		t.Fatalf("tool_use not surfaced: %+v", got.ToolCalls)
	}
}

// ── Gemini ─────────────────────────────────────────────────────────────────────

func TestGemini_SuggestReply(t *testing.T) {
	resp := `{"candidates":[{"content":{"parts":[{"text":"Hi from Gemini"}]}}],"usageMetadata":{"promptTokenCount":11,"candidatesTokenCount":5}}`
	srv, cap := captureServer(t, resp)
	got, err := NewGemini().Infer(t.Context(), suggestReq(srv.URL))
	if err != nil {
		t.Fatalf("infer: %v", err)
	}
	if got.Text != "Hi from Gemini" || got.TokensInput != 11 || got.TokensOutput != 5 {
		t.Fatalf("unexpected response: %+v", got)
	}
	if !strings.Contains(cap.path, ":generateContent") {
		t.Errorf("unexpected path %q", cap.path)
	}
	if !strings.Contains(cap.query, "key=sk-test") {
		t.Errorf("api key not sent as query param: %q", cap.query)
	}
}

// ── error handling ─────────────────────────────────────────────────────────────

func TestProviders_NoKey_ReturnsError(t *testing.T) {
	for name, p := range map[string]contracts.AIProvider{
		"openai": NewOpenAI(), "anthropic": NewAnthropic(), "gemini": NewGemini(),
		"mistral": NewMistral(), "deepseek": NewDeepSeek(), "perplexity": NewPerplexity(),
	} {
		req := suggestReq("http://127.0.0.1:0")
		req.APIKey = ""
		if _, err := p.Infer(t.Context(), req); err == nil {
			t.Errorf("%s: expected an error with no API key", name)
		}
	}
}

func TestOpenAI_HTTPErrorIsReturned(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid key"}`))
	}))
	t.Cleanup(srv.Close)
	if _, err := NewOpenAI().Infer(t.Context(), suggestReq(srv.URL)); err == nil {
		t.Fatal("expected an error on HTTP 401")
	}
}

// ── registry: echo excluded from production ────────────────────────────────────

func TestRegistry_ResolvesRealProvidersNotEcho(t *testing.T) {
	reg := NewRegistry()
	for _, p := range []entity.Provider{
		entity.ProviderOpenAI, entity.ProviderAnthropic, entity.ProviderGemini,
		entity.ProviderMistral, entity.ProviderDeepSeek, entity.ProviderPerplexity,
	} {
		if _, err := reg.Resolve(p); err != nil {
			t.Errorf("registry should resolve %q: %v", p, err)
		}
	}
	if _, err := reg.Resolve(entity.Provider("echo")); err == nil {
		t.Error("echo must NOT be resolvable in production")
	}
}

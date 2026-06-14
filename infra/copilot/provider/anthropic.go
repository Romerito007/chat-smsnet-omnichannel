package provider

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
)

// anthropicVersion is the required Anthropic API version header.
const anthropicVersion = "2023-06-01"

// Anthropic is the Anthropic Messages API adapter. It is stateless; the API key
// and optional base URL arrive per-request from the tenant's AIConfig.
type Anthropic struct {
	defaultBaseURL string
	defaultModel   string
}

// NewAnthropic builds the adapter.
func NewAnthropic() *Anthropic {
	return &Anthropic{defaultBaseURL: "https://api.anthropic.com", defaultModel: "claude-3-5-haiku-latest"}
}

// Name implements contracts.AIProvider.
func (a *Anthropic) Name() string { return string(entity.ProviderAnthropic) }

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema,omitempty"`
}

type anthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature"`
	System      string             `json:"system,omitempty"`
	Messages    []anthropicMessage `json:"messages"`
	Tools       []anthropicTool    `json:"tools,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string, or []block for tool_use / tool_result
}

type anthropicBlock struct {
	Type string `json:"type"`
	// tool_use
	ID    string         `json:"id,omitempty"`
	Name  string         `json:"name,omitempty"`
	Input map[string]any `json:"input,omitempty"`
	// tool_result
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
}

type anthropicResponse struct {
	Content []struct {
		Type  string          `json:"type"`
		Text  string          `json:"text"`
		ID    string          `json:"id"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// Infer implements contracts.AIProvider against the Messages API.
func (a *Anthropic) Infer(ctx context.Context, req contracts.Request) (contracts.Response, error) {
	if req.APIKey == "" {
		return contracts.Response{}, notConfigured(entity.ProviderAnthropic)
	}
	base := orDefault(req.BaseURL, a.defaultBaseURL)
	messages := []anthropicMessage{{Role: "user", Content: renderContext(req.Context)}}
	// Replay the tool-calling loop: assistant tool_use blocks then a user turn of
	// tool_result blocks, per exchange.
	for _, ex := range req.ToolHistory {
		use := make([]anthropicBlock, 0, len(ex.Calls))
		for _, c := range ex.Calls {
			use = append(use, anthropicBlock{Type: "tool_use", ID: c.ID, Name: c.Name, Input: rawToMap(c.Arguments)})
		}
		messages = append(messages, anthropicMessage{Role: "assistant", Content: use})
		results := make([]anthropicBlock, 0, len(ex.Results))
		for _, res := range ex.Results {
			results = append(results, anthropicBlock{Type: "tool_result", ToolUseID: res.ID, Content: res.Content})
		}
		messages = append(messages, anthropicMessage{Role: "user", Content: results})
	}
	payload := anthropicRequest{
		Model:       orDefault(req.Model, a.defaultModel),
		MaxTokens:   maxTokensOr(req.MaxTokens, 1024), // Messages API requires max_tokens
		Temperature: req.Temperature,
		System:      fullSystemPrompt(req),
		Messages:    messages,
		Tools:       toAnthropicTools(req.Tools),
	}

	body, err := postJSON(ctx, base+"/v1/messages", map[string]string{
		"x-api-key":         req.APIKey,
		"anthropic-version": anthropicVersion,
	}, payload)
	if err != nil {
		return contracts.Response{}, err
	}

	var parsed anthropicResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return contracts.Response{}, err
	}
	var text strings.Builder
	out := contracts.Response{
		TokensInput:  parsed.Usage.InputTokens,
		TokensOutput: parsed.Usage.OutputTokens,
	}
	for _, block := range parsed.Content {
		switch block.Type {
		case "text":
			text.WriteString(block.Text)
		case "tool_use":
			out.ToolCalls = append(out.ToolCalls, contracts.ToolCall{
				ID: block.ID, Name: block.Name, Arguments: string(block.Input),
			})
		}
	}
	out.Text = text.String()
	if req.Action == entity.ActionClassify {
		out.Categories = classifyCategories(out.Text, req.Context.Instruction)
	}
	return out, nil
}

// rawToMap parses a JSON object string into a map; invalid/empty input yields an
// empty object so the replayed tool_use block is always well-formed.
func rawToMap(s string) map[string]any {
	out := map[string]any{}
	if strings.TrimSpace(s) == "" {
		return out
	}
	_ = json.Unmarshal([]byte(s), &out)
	return out
}

func toAnthropicTools(tools []contracts.ToolDefinition) []anthropicTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]anthropicTool, 0, len(tools))
	for _, t := range tools {
		out = append(out, anthropicTool{Name: t.Name, Description: t.Description, InputSchema: t.Schema})
	}
	return out
}

var _ contracts.AIProvider = (*Anthropic)(nil)

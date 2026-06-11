package provider

import (
	"context"
	"encoding/json"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
)

// openAICompatible is the shared adapter for providers that speak the OpenAI
// Chat Completions API: OpenAI itself plus Mistral, DeepSeek and Perplexity. The
// only differences are the default base URL and model, so each concrete provider
// is a thin constructor over this type. Credentials (API key, optional base URL)
// arrive per-request from the tenant's AIConfig — the adapter is stateless.
type openAICompatible struct {
	provider       entity.Provider
	defaultBaseURL string
	defaultModel   string
}

// Name implements contracts.AIProvider.
func (o *openAICompatible) Name() string { return string(o.provider) }

type oaiMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content"`
	ToolCalls  []oaiToolCallReq `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type oaiToolCallReq struct {
	ID       string        `json:"id"`
	Type     string        `json:"type"`
	Function oaiToolCallFn `json:"function"`
}

type oaiToolCallFn struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type oaiTool struct {
	Type     string      `json:"type"`
	Function oaiFunction `json:"function"`
}

type oaiFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type oaiRequest struct {
	Model       string       `json:"model"`
	Messages    []oaiMessage `json:"messages"`
	Temperature float64      `json:"temperature"`
	MaxTokens   int          `json:"max_tokens,omitempty"`
	Tools       []oaiTool    `json:"tools,omitempty"`
}

type oaiResponse struct {
	Choices []struct {
		Message struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// Infer implements contracts.AIProvider against the Chat Completions endpoint.
func (o *openAICompatible) Infer(ctx context.Context, req contracts.Request) (contracts.Response, error) {
	if req.APIKey == "" {
		return contracts.Response{}, notConfigured(o.provider)
	}
	base := orDefault(req.BaseURL, o.defaultBaseURL)
	messages := []oaiMessage{
		{Role: "system", Content: systemPrompt(req.Action)},
		{Role: "user", Content: renderContext(req.Context)},
	}
	// Replay the tool-calling loop: each exchange is an assistant turn with
	// tool_calls followed by a tool message per result.
	for _, ex := range req.ToolHistory {
		calls := make([]oaiToolCallReq, 0, len(ex.Calls))
		for _, c := range ex.Calls {
			calls = append(calls, oaiToolCallReq{ID: c.ID, Type: "function", Function: oaiToolCallFn{Name: c.Name, Arguments: c.Arguments}})
		}
		messages = append(messages, oaiMessage{Role: "assistant", ToolCalls: calls})
		for _, res := range ex.Results {
			messages = append(messages, oaiMessage{Role: "tool", ToolCallID: res.ID, Content: res.Content})
		}
	}
	payload := oaiRequest{
		Model:       orDefault(req.Model, o.defaultModel),
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   maxTokensOr(req.MaxTokens, 0),
		Tools:       toOAITools(req.Tools),
	}

	body, err := postJSON(ctx, base+"/chat/completions", map[string]string{
		"Authorization": "Bearer " + req.APIKey,
	}, payload)
	if err != nil {
		return contracts.Response{}, err
	}

	var parsed oaiResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return contracts.Response{}, err
	}
	out := contracts.Response{
		TokensInput:  parsed.Usage.PromptTokens,
		TokensOutput: parsed.Usage.CompletionTokens,
	}
	if len(parsed.Choices) > 0 {
		msg := parsed.Choices[0].Message
		out.Text = msg.Content
		for _, tc := range msg.ToolCalls {
			out.ToolCalls = append(out.ToolCalls, contracts.ToolCall{
				ID: tc.ID, Name: tc.Function.Name, Arguments: tc.Function.Arguments,
			})
		}
	}
	if req.Action == entity.ActionClassify {
		out.Categories = classifyCategories(out.Text, req.Context.Instruction)
	}
	return out, nil
}

func toOAITools(tools []contracts.ToolDefinition) []oaiTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]oaiTool, 0, len(tools))
	for _, t := range tools {
		out = append(out, oaiTool{
			Type:     "function",
			Function: oaiFunction{Name: t.Name, Description: t.Description, Parameters: t.Schema},
		})
	}
	return out
}

// ── concrete OpenAI-compatible providers ──────────────────────────────────────

// NewOpenAI builds the OpenAI adapter.
func NewOpenAI() contracts.AIProvider {
	return &openAICompatible{provider: entity.ProviderOpenAI, defaultBaseURL: "https://api.openai.com/v1", defaultModel: "gpt-4o-mini"}
}

// NewMistral builds the Mistral adapter (OpenAI-compatible).
func NewMistral() contracts.AIProvider {
	return &openAICompatible{provider: entity.ProviderMistral, defaultBaseURL: "https://api.mistral.ai/v1", defaultModel: "mistral-small-latest"}
}

// NewDeepSeek builds the DeepSeek adapter (OpenAI-compatible).
func NewDeepSeek() contracts.AIProvider {
	return &openAICompatible{provider: entity.ProviderDeepSeek, defaultBaseURL: "https://api.deepseek.com/v1", defaultModel: "deepseek-chat"}
}

// NewPerplexity builds the Perplexity adapter (OpenAI-compatible).
func NewPerplexity() contracts.AIProvider {
	return &openAICompatible{provider: entity.ProviderPerplexity, defaultBaseURL: "https://api.perplexity.ai", defaultModel: "sonar"}
}

var _ contracts.AIProvider = (*openAICompatible)(nil)

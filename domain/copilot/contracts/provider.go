// Package contracts holds the copilot ports (the provider-agnostic AIProvider
// interface and optional context data sources) plus the service inputs/results.
package contracts

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
)

// AIProvider is the provider-agnostic inference port. Every backend (openai,
// anthropic, gemini, mistral, deepseek, perplexity) implements it; the domain
// never depends on a concrete provider.
type AIProvider interface {
	// Name returns the provider identifier (used in logs).
	Name() string
	// Infer runs one inference over an already policy-filtered context.
	Infer(ctx context.Context, req Request) (Response, error)
}

// Request is one inference request. The Context has already been filtered by the
// tenant's allow_*_data policies before reaching the provider. APIKey/BaseURL are
// the per-tenant credentials resolved from the AIConfig (the key decrypted in
// memory); a provider with no key returns a friendly "not configured" error.
type Request struct {
	Action      entity.Action
	Model       string
	APIKey      string
	BaseURL     string
	Temperature float64
	MaxTokens   int
	// SystemInstructions is the assistant's free-text persona/conduct, APPENDED to
	// the fixed action system prompt (never replacing it). Empty for none.
	SystemInstructions string
	Context            PromptContext
	// Tools are the (provider-agnostic) tool/function definitions the model may
	// call. They come from the MCP registry, not from any hard-coded tool. When
	// empty, the provider runs a plain completion.
	Tools []ToolDefinition
	// ToolHistory replays the tool-calling loop so far: each exchange is one
	// assistant turn's tool calls plus their results, fed back so the model can
	// continue toward a final answer.
	ToolHistory []ToolExchange
}

// ToolExchange is one round of the tool-calling loop: the calls the model made
// and the results the chat fed back.
type ToolExchange struct {
	Calls   []ToolCall
	Results []ToolResult
}

// ToolResult is the outcome of executing (or proposing) one tool call, fed back
// to the model. Content carries no secrets.
type ToolResult struct {
	ID      string
	Name    string
	Content string
}

// Response is the provider's normalized output.
type Response struct {
	Text         string
	Categories   []string // populated for classify
	TokensInput  int
	TokensOutput int
	// ToolCalls are the tool invocations the model requested. The caller executes
	// read-only tools and surfaces write tools for human approval; this is the
	// provider-side half of the tool-calling loop.
	ToolCalls []ToolCall
}

// ToolDefinition is a provider-agnostic tool/function the model may call. Schema
// is the JSON-Schema of the arguments. ReadOnly marks tools the AI may invoke
// directly; non-read-only (write) tools are only proposed for human approval.
type ToolDefinition struct {
	Name        string
	Description string
	Schema      map[string]any
	ReadOnly    bool
}

// ToolCall is a tool invocation requested by the model.
type ToolCall struct {
	ID        string
	Name      string
	Arguments string // raw JSON arguments as produced by the model
}

// PromptContext is the policy-filtered context handed to a provider. The customer
// profile is gated by allow_customer_data (nil when forbidden); financial and
// monitoring data are consulted on demand via ISP tools, never pre-injected.
type PromptContext struct {
	Channel     string
	Transcript  []Turn
	Instruction string        // action-specific guidance (e.g. classify categories)
	Customer    *CustomerInfo // nil unless allow_customer_data
	// AgentChat is the AGENT↔assistant side chat (role agent|assistant), DISTINCT
	// from Transcript (the customer conversation). Only set for the agent_chat
	// action, so the assistant remembers the agent's earlier questions.
	AgentChat []Turn
}

// Turn is one message in the conversation transcript.
type Turn struct {
	Role string // customer | agent | automation | system
	Text string
}

// CustomerInfo is the customer profile subset (gated by allow_customer_data).
type CustomerInfo struct {
	Name     string
	Document string
	Phone    string
}

// Package contracts holds the copilot ports (the provider-agnostic AIProvider
// interface and optional context data sources) plus the service inputs/results.
package contracts

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
)

// AIProvider is the provider-agnostic inference port. Every backend (echo,
// openai, gemini, anthropic, failover) implements it; the domain never depends
// on a concrete provider.
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
	Context     PromptContext
	// Tools are the (provider-agnostic) tool/function definitions the model may
	// call. They come from the MCP registry, not from any hard-coded tool. When
	// empty, the provider runs a plain completion.
	Tools []ToolDefinition
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

// PromptContext is the policy-filtered context handed to a provider. Sections
// gated by an allow_*_data flag are nil when the policy forbids them, so a
// provider can never receive disallowed data.
type PromptContext struct {
	Channel     string
	Transcript  []Turn
	Instruction string          // action-specific guidance (e.g. classify categories)
	Customer    *CustomerInfo   // nil unless allow_customer_data
	Financial   *FinancialInfo  // nil unless allow_financial_data
	Monitoring  *MonitoringInfo // nil unless allow_monitoring_data
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

// FinancialInfo is a financial summary (gated by allow_financial_data).
type FinancialInfo struct {
	Summary string
}

// MonitoringInfo is a technical-status summary (gated by allow_monitoring_data).
type MonitoringInfo struct {
	Summary string
}

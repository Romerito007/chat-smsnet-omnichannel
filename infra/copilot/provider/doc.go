// Package provider holds the copilot AI adapters behind the domain's AIProvider
// port. Production ships real HTTP adapters: openai, mistral, deepseek and
// perplexity (all OpenAI Chat Completions-compatible, sharing openAICompatible),
// anthropic (Messages API) and gemini (generateContent). Adapters are stateless —
// the per-tenant API key and optional base URL travel on each Request — so one
// registry serves every tenant. They support the tool-calling loop: tool
// definitions (from the MCP registry, never hard-coded) are forwarded and the
// model's tool calls are surfaced for the caller to execute (read tools) or
// propose for approval (write tools).
//
// The echo provider is a deterministic mock kept for tests only; it is never
// wired into the production registry.
package provider

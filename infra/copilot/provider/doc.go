// Package provider holds the copilot AI adapters behind the domain's AIProvider
// port. Production ships real HTTP adapters: openai, mistral, deepseek and
// perplexity (all OpenAI Chat Completions-compatible, sharing openAICompatible),
// anthropic (Messages API) and gemini (generateContent). Adapters are stateless —
// the per-tenant API key and optional base URL travel on each Request — so one
// registry serves every tenant. They support the tool-calling loop: tool
// definitions (from the MCP registry, never hard-coded) are forwarded and the
// model's tool calls are surfaced for the caller to execute (read tools) or
// propose for approval (write tools). Only real hosted providers are registered;
// any deterministic test double lives under *_test.go.
package provider

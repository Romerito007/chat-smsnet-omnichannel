// Package entity holds the copilot domain entities: the per-tenant AI config and
// the AI usage log.
package entity

import "time"

// Provider identifies the configured AI backend. Every value below is a real
// hosted provider with an adapter behind the AIProvider port.
type Provider string

const (
	ProviderOpenAI     Provider = "openai"
	ProviderAnthropic  Provider = "anthropic"
	ProviderGemini     Provider = "gemini"
	ProviderMistral    Provider = "mistral"
	ProviderDeepSeek   Provider = "deepseek"
	ProviderPerplexity Provider = "perplexity"
)

// IsValidProvider reports whether p is a selectable production provider.
func IsValidProvider(p Provider) bool {
	switch p {
	case ProviderOpenAI, ProviderAnthropic, ProviderGemini,
		ProviderMistral, ProviderDeepSeek, ProviderPerplexity:
		return true
	default:
		return false
	}
}

// AIConfig is a tenant's copilot AI INFRASTRUCTURE: the shared provider, model,
// credential and endpoint. Behavior (privacy gates, human approval, sampling,
// persona) lives per-assistant on CopilotAssistant — see entity.Behavior — so a
// single AI key serves segments with different conduct.
type AIConfig struct {
	ID       string
	TenantID string
	Provider Provider
	Model    string
	// APIKey is the per-tenant provider credential. It is held in plaintext in
	// memory but stored encrypted at rest (AES-GCM) and never returned to clients.
	APIKey string
	// BaseURL optionally overrides the provider's default API endpoint (e.g. an
	// OpenAI-compatible gateway or a self-hosted proxy).
	BaseURL   string
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

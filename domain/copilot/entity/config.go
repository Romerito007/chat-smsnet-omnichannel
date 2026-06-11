// Package entity holds the copilot domain entities: the per-tenant AI config and
// the AI usage log.
package entity

import "time"

// Provider identifies the configured AI backend. Production ships real adapters
// behind the AIProvider port; echo is a deterministic mock kept for tests only
// and is deliberately NOT a valid (selectable) production provider.
type Provider string

const (
	ProviderOpenAI     Provider = "openai"
	ProviderAnthropic  Provider = "anthropic"
	ProviderGemini     Provider = "gemini"
	ProviderMistral    Provider = "mistral"
	ProviderDeepSeek   Provider = "deepseek"
	ProviderPerplexity Provider = "perplexity"

	// ProviderEcho is the test-only mock. It is never wired into production.
	ProviderEcho Provider = "echo"
	// ProviderFailover is retained for compatibility but is not selectable.
	ProviderFailover Provider = "failover"
)

// IsValidProvider reports whether p is a selectable production provider. Echo and
// failover are intentionally excluded so they cannot be configured by a tenant.
func IsValidProvider(p Provider) bool {
	switch p {
	case ProviderOpenAI, ProviderAnthropic, ProviderGemini,
		ProviderMistral, ProviderDeepSeek, ProviderPerplexity:
		return true
	default:
		return false
	}
}

// AIConfig is a tenant's copilot configuration. The allow_*_data flags are
// privacy switches: when false, the corresponding data is never placed in the
// prompt sent to the provider, regardless of availability.
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
	BaseURL               string
	Temperature           float64
	MaxTokens             int
	AllowCustomerData     bool
	AllowFinancialData    bool
	AllowMonitoringData   bool
	HumanApprovalRequired bool
	Enabled               bool
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

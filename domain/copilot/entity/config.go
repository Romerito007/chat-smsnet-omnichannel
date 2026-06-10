// Package entity holds the copilot domain entities: the per-tenant AI config and
// the AI usage log.
package entity

import "time"

// Provider identifies the configured AI backend. The MVP ships a functional
// mock (echo); the others are pluggable adapters behind the AIProvider port.
type Provider string

const (
	ProviderEcho      Provider = "echo"
	ProviderOpenAI    Provider = "openai"
	ProviderGemini    Provider = "gemini"
	ProviderAnthropic Provider = "anthropic"
	ProviderFailover  Provider = "failover"
)

// IsValidProvider reports whether p is a known provider.
func IsValidProvider(p Provider) bool {
	switch p {
	case ProviderEcho, ProviderOpenAI, ProviderGemini, ProviderAnthropic, ProviderFailover:
		return true
	default:
		return false
	}
}

// AIConfig is a tenant's copilot configuration. The allow_*_data flags are
// privacy switches: when false, the corresponding data is never placed in the
// prompt sent to the provider, regardless of availability.
type AIConfig struct {
	ID                    string
	TenantID              string
	Provider              Provider
	Model                 string
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

package provider

import (
	"fmt"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
)

// Keys holds the per-environment API keys for the hosted providers. Empty keys
// leave the corresponding adapter unconfigured (it reports as such); the echo
// provider needs no key.
type Keys struct {
	OpenAI    string
	Gemini    string
	Anthropic string
}

// Registry resolves a configured provider name to its adapter. It implements the
// domain's ProviderResolver port without the domain knowing any concrete
// provider.
type Registry struct {
	adapters map[entity.Provider]contracts.AIProvider
}

// NewRegistry builds the registry with the echo provider always available, the
// hosted adapters built from their keys, and a failover that prefers the hosted
// providers and falls back to echo.
func NewRegistry(keys Keys) *Registry {
	echo := NewEcho()
	openai := NewOpenAI(keys.OpenAI)
	gemini := NewGemini(keys.Gemini)
	anthropic := NewAnthropic(keys.Anthropic)

	return &Registry{adapters: map[entity.Provider]contracts.AIProvider{
		entity.ProviderEcho:      echo,
		entity.ProviderOpenAI:    openai,
		entity.ProviderGemini:    gemini,
		entity.ProviderAnthropic: anthropic,
		// Failover order: hosted providers first, echo as the safe last resort.
		entity.ProviderFailover: NewFailover(openai, gemini, anthropic, echo),
	}}
}

// Resolve implements contracts.ProviderResolver.
func (r *Registry) Resolve(p entity.Provider) (contracts.AIProvider, error) {
	if adapter, ok := r.adapters[p]; ok {
		return adapter, nil
	}
	return nil, fmt.Errorf("unknown provider %q", p)
}

var _ contracts.ProviderResolver = (*Registry)(nil)

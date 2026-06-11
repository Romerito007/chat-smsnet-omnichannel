package provider

import (
	"fmt"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
)

// Registry resolves a configured provider name to its real adapter. It implements
// the domain's ProviderResolver port without the domain knowing any concrete
// provider. Adapters are stateless: the per-tenant API key and base URL travel on
// each Request, so a single registry instance serves every tenant.
//
// The echo mock is deliberately NOT registered: it exists for tests only and is
// never selectable in production.
type Registry struct {
	adapters map[entity.Provider]contracts.AIProvider
}

// NewRegistry builds the registry with every real provider adapter.
func NewRegistry() *Registry {
	return &Registry{adapters: map[entity.Provider]contracts.AIProvider{
		entity.ProviderOpenAI:     NewOpenAI(),
		entity.ProviderAnthropic:  NewAnthropic(),
		entity.ProviderGemini:     NewGemini(),
		entity.ProviderMistral:    NewMistral(),
		entity.ProviderDeepSeek:   NewDeepSeek(),
		entity.ProviderPerplexity: NewPerplexity(),
	}}
}

// Resolve implements contracts.ProviderResolver. An unknown or non-production
// provider (e.g. echo) yields an error so the service surfaces a friendly message.
func (r *Registry) Resolve(p entity.Provider) (contracts.AIProvider, error) {
	if adapter, ok := r.adapters[p]; ok {
		return adapter, nil
	}
	return nil, fmt.Errorf("unknown or unavailable provider %q", p)
}

var _ contracts.ProviderResolver = (*Registry)(nil)

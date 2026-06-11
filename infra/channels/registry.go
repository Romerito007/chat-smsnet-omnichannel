// Package channels wires the channel adapters into a registry resolved by type.
package channels

import (
	chcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/contracts"
	chentity "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/channels/api"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/channels/webchat"
)

// Registry resolves an adapter for a channel type, falling back to the generic
// API channel adapter for types without a dedicated one.
type Registry struct {
	adapters map[chentity.Type]chcontracts.Adapter
	fallback chcontracts.Adapter
}

// NewRegistry builds the registry with the real adapters. The generic API channel
// is the production default; webchat acknowledges delivery over the realtime
// layer. Every other type (whatsapp/telegram/instagram/custom) integrates over
// the generic API channel until it gets a dedicated adapter.
func NewRegistry() *Registry {
	return &Registry{
		adapters: map[chentity.Type]chcontracts.Adapter{
			chentity.TypeAPI:     api.New(chentity.TypeAPI),
			chentity.TypeWebchat: webchat.New(),
		},
		fallback: api.New(chentity.TypeCustom),
	}
}

// For resolves the adapter for a type.
func (r *Registry) For(t chentity.Type) chcontracts.Adapter {
	if a, ok := r.adapters[t]; ok {
		return a
	}
	return r.fallback
}

var _ chcontracts.AdapterRegistry = (*Registry)(nil)

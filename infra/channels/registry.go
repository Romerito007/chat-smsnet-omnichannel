// Package channels wires the channel adapters into a registry resolved by type.
package channels

import (
	chcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/contracts"
	chentity "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/channels/api"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/channels/webchat"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/channels/whatsapp"
)

// Registry resolves an adapter for a channel type, falling back to the generic
// API channel adapter for types without a dedicated one.
type Registry struct {
	adapters map[chentity.Type]chcontracts.Adapter
	fallback chcontracts.Adapter
}

// NewRegistry builds the registry with the available adapters. No mock adapter
// is wired in production: the generic API channel is the real default.
func NewRegistry() *Registry {
	return &Registry{
		adapters: map[chentity.Type]chcontracts.Adapter{
			chentity.TypeAPI:      api.New(chentity.TypeAPI),
			chentity.TypeWhatsApp: whatsapp.New(),
			chentity.TypeWebchat:  webchat.New(),
		},
		// telegram / instagram / custom integrate over the generic API channel.
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

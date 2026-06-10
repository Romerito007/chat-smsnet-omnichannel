// Package channels wires the channel adapters into a registry resolved by type.
package channels

import (
	chcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/contracts"
	chentity "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/channels/mock"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/channels/webchat"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/channels/whatsapp"
)

// Registry resolves an adapter for a channel type, falling back to the
// functional mock for types without a dedicated adapter.
type Registry struct {
	adapters map[chentity.Type]chcontracts.Adapter
	fallback chcontracts.Adapter
}

// NewRegistry builds the registry with the available adapters.
func NewRegistry() *Registry {
	return &Registry{
		adapters: map[chentity.Type]chcontracts.Adapter{
			chentity.TypeWhatsApp: whatsapp.New(),
			chentity.TypeWebchat:  webchat.New(),
		},
		// telegram / instagram / custom use the functional mock for now.
		fallback: mock.New(chentity.TypeCustom),
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

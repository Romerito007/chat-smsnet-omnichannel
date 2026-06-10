package contracts

import "github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"

// ProviderResolver maps a configured provider name to its AIProvider adapter.
// The infra registry implements it; an unknown or unconfigured provider yields
// an error so the service can surface a friendly message.
type ProviderResolver interface {
	Resolve(p entity.Provider) (AIProvider, error)
}

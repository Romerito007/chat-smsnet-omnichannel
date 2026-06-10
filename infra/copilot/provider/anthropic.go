package provider

import (
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
)

// Anthropic is the Anthropic adapter (pluggable; activated when an API key is set).
type Anthropic struct{ remote }

// NewAnthropic builds the adapter.
func NewAnthropic(apiKey string) *Anthropic {
	return &Anthropic{remote{name: entity.ProviderAnthropic, apiKey: apiKey}}
}

var _ contracts.AIProvider = (*Anthropic)(nil)

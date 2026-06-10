package provider

import (
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
)

// OpenAI is the OpenAI adapter (pluggable; activated when an API key is set).
type OpenAI struct{ remote }

// NewOpenAI builds the adapter.
func NewOpenAI(apiKey string) *OpenAI {
	return &OpenAI{remote{name: entity.ProviderOpenAI, apiKey: apiKey}}
}

var _ contracts.AIProvider = (*OpenAI)(nil)

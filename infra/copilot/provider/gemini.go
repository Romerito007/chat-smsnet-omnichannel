package provider

import (
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
)

// Gemini is the Google Gemini adapter (pluggable; activated when an API key is set).
type Gemini struct{ remote }

// NewGemini builds the adapter.
func NewGemini(apiKey string) *Gemini {
	return &Gemini{remote{name: entity.ProviderGemini, apiKey: apiKey}}
}

var _ contracts.AIProvider = (*Gemini)(nil)

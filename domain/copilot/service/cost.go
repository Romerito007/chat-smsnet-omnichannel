package service

import "github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"

// costPer1kTokens is an indicative blended USD cost per 1k tokens per provider,
// used only to populate AILog.estimated_cost for observability/billing. The mock
// provider is free.
var costPer1kTokens = map[entity.Provider]float64{
	entity.ProviderEcho:      0.0,
	entity.ProviderOpenAI:    0.005,
	entity.ProviderGemini:    0.002,
	entity.ProviderAnthropic: 0.006,
	entity.ProviderFailover:  0.006,
}

// estimateCost returns the indicative cost for a call given token counts.
func estimateCost(provider entity.Provider, tokensIn, tokensOut int) float64 {
	rate := costPer1kTokens[provider]
	if rate == 0 {
		return 0
	}
	return rate * float64(tokensIn+tokensOut) / 1000.0
}

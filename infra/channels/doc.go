// Package channels holds the outbound/inbound channel adapters (whatsapp,
// telegram, webchat, mock). Each adapter implements a common Channel port so the
// messaging domain stays provider-agnostic. Delivery is driven by the Asynq
// `channels` queue (channel.deliver / channel.retry).
//
// This package is a placeholder in the foundation; concrete adapters are added
// alongside the messaging domain.
package channels

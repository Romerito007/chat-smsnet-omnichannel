// Package channels holds the outbound/inbound channel adapters (the generic
// `api` channel, whatsapp, webchat) and the registry that resolves one by type.
// Each adapter implements the common contracts.Adapter port so the messaging
// domain stays provider-agnostic. Outbound delivery is driven by the Asynq
// `channels` queue (channel.deliver / channel.retry). Shared HMAC signing and
// receipt parsing live in the leaf `sign` subpackage.
package channels

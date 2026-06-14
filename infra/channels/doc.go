// Package channels holds the inbound channel adapters (the generic `api` channel,
// whatsapp, webchat) and the registry that resolves one by type. Each adapter
// implements the common contracts.Adapter port so the messaging domain stays
// provider-agnostic. Outbound message delivery is NOT a separate channel rail: an
// agent's reply reaches the customer through the message_created webhook (the
// channel auto-manages a webhook subscription from its outbound URL). Shared HMAC
// signing lives in the leaf `sign` subpackage.
package channels

// Package provider holds the copilot AI adapters behind the domain's AIProvider
// port: echo (a functional mock used in the MVP), openai, gemini and anthropic
// (pluggable hosted backends, activated when an API key is set), and failover
// (tries an ordered list, falling back to echo). The domain depends only on the
// port, never on a concrete provider, so backends can be swapped per tenant.
package provider

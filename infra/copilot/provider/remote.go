package provider

import (
	"context"
	"fmt"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
)

// remote is the shared skeleton for the hosted providers (openai, gemini,
// anthropic). The MVP ships the echo provider as the functional backend; these
// adapters are wired only when an API key is configured for the environment.
// Until a key is present, Infer reports the provider as unconfigured so the
// resolver/failover can react, rather than making a half-configured call.
type remote struct {
	name   entity.Provider
	apiKey string
}

// Configured reports whether the adapter has credentials.
func (r *remote) Configured() bool { return r.apiKey != "" }

// Name implements AIProvider.
func (r *remote) Name() string { return string(r.name) }

// Infer implements AIProvider. The real HTTP integration is intentionally not
// part of the MVP; with a key set this is where the provider-specific request
// would be issued against renderContext(req.Context).
func (r *remote) Infer(_ context.Context, _ contracts.Request) (contracts.Response, error) {
	if !r.Configured() {
		return contracts.Response{}, fmt.Errorf("provider %s is not configured (no API key)", r.name)
	}
	return contracts.Response{}, fmt.Errorf("provider %s is configured but not enabled in this build", r.name)
}

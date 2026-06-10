package provider

import (
	"context"
	"errors"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
)

// Failover tries an ordered list of providers, returning the first successful
// response. It lets a tenant prefer a hosted provider while always falling back
// to a working backend (the echo mock is the safe last resort).
type Failover struct {
	providers []contracts.AIProvider
}

// NewFailover builds the failover provider from an ordered list. nil entries are
// ignored.
func NewFailover(providers ...contracts.AIProvider) *Failover {
	out := make([]contracts.AIProvider, 0, len(providers))
	for _, p := range providers {
		if p != nil {
			out = append(out, p)
		}
	}
	return &Failover{providers: out}
}

// Name implements AIProvider.
func (f *Failover) Name() string { return string(entity.ProviderFailover) }

// Infer tries each provider in order, returning the first success.
func (f *Failover) Infer(ctx context.Context, req contracts.Request) (contracts.Response, error) {
	var lastErr error
	for _, p := range f.providers {
		resp, err := p.Infer(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("no providers configured")
	}
	return contracts.Response{}, lastErr
}

var _ contracts.AIProvider = (*Failover)(nil)

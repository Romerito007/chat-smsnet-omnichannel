package contracts

import (
	"context"

	chentity "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
)

// ConnectionHealthChecker probes a channel connection's reachability. The infra
// implementation does a lightweight HTTP check for connections with a base URL;
// connections without a remote endpoint are treated as healthy. Used by the
// channels.health_check job.
type ConnectionHealthChecker interface {
	Check(ctx context.Context, conn *chentity.ChannelConnection) error
}

// NoopHealthChecker reports every connection healthy.
type NoopHealthChecker struct{}

// Check implements ConnectionHealthChecker.
func (NoopHealthChecker) Check(context.Context, *chentity.ChannelConnection) error { return nil }

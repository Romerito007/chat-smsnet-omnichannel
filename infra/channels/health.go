package channels

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	chcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/contracts"
	chentity "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	infrahttp "github.com/romerito007/chat-smsnet-omnichannel/infra/http_client"
)

// HealthChecker probes a channel connection's reachability with a lightweight
// HTTP request to its base URL. A connection without a base URL has no remote
// endpoint to probe and is reported healthy.
type HealthChecker struct {
	client *http.Client
}

// NewHealthChecker builds the checker.
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{client: infrahttp.New(5 * time.Second)}
}

// Check returns nil when the connection is reachable, an error otherwise. Any
// HTTP response (even 4xx/5xx) counts as reachable — the endpoint is up; only a
// transport failure (DNS/connection/timeout) marks the connection unhealthy.
func (h *HealthChecker) Check(ctx context.Context, conn *chentity.ChannelConnection) error {
	base := strings.TrimSpace(conn.BaseURL)
	if base == "" {
		return nil // nothing remote to probe
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+"/", nil)
	if err != nil {
		return err
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("channel %s unreachable: %w", conn.ID, err)
	}
	_ = resp.Body.Close()
	return nil
}

var _ chcontracts.ConnectionHealthChecker = (*HealthChecker)(nil)

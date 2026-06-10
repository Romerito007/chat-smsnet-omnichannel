// Package monitoring is the HTTP client for the tenant's external monitoring
// system. It is on-demand only: no caching, no persistence, no realtime
// ingestion. The normalized response is returned and discarded after the request;
// metadata is filtered to a safe allow-list.
package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	mcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/monitoring/contracts"
	mentity "github.com/romerito007/chat-smsnet-omnichannel/domain/monitoring/entity"
	infrahttp "github.com/romerito007/chat-smsnet-omnichannel/infra/http_client"
)

// allowedMetadata is the safe allow-list of metadata keys passed through to the
// client. Anything else from the external payload is dropped.
var allowedMetadata = map[string]struct{}{
	"region": {}, "device": {}, "uptime": {}, "latency_ms": {},
	"packet_loss": {}, "plan": {}, "technology": {}, "signal_dbm": {},
}

// Gateway is the HTTP implementation of the monitoring gateway.
type Gateway struct {
	client *http.Client
}

// NewGateway builds the gateway.
func NewGateway() *Gateway {
	return &Gateway{client: infrahttp.New(15 * time.Second)}
}

// rawSummary mirrors the external response before metadata filtering.
type rawSummary struct {
	mcontracts.MonitoringSummary
	Metadata map[string]any `json:"metadata"`
}

func (g *Gateway) GetSummary(ctx context.Context, cfg *mentity.MonitoringIntegrationConfig, lk mcontracts.Lookup) (mcontracts.MonitoringSummary, error) {
	var raw rawSummary
	if err := g.get(ctx, cfg, "/monitoring/summary", lk, &raw); err != nil {
		return mcontracts.MonitoringSummary{}, err
	}
	out := raw.MonitoringSummary
	out.Metadata = filterMetadata(raw.Metadata)
	out.CustomerStatus = normalizeStatus(out.CustomerStatus)
	out.Severity = normalizeSeverity(out.Severity)
	return out, nil
}

func (g *Gateway) GetIncidents(ctx context.Context, cfg *mentity.MonitoringIntegrationConfig, lk mcontracts.Lookup) ([]mcontracts.Incident, error) {
	var out []mcontracts.Incident
	if err := g.get(ctx, cfg, "/monitoring/incidents", lk, &out); err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Severity = normalizeSeverity(out[i].Severity)
	}
	return out, nil
}

// Ping verifies connectivity (config test).
func (g *Gateway) Ping(ctx context.Context, cfg *mentity.MonitoringIntegrationConfig) error {
	reqCtx, cancel := g.withTimeout(ctx, cfg)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, strings.TrimRight(cfg.BaseURL, "/")+"/health", nil)
	if err != nil {
		return err
	}
	g.auth(req, cfg)
	resp, err := g.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("monitoring returned status %d", resp.StatusCode)
	}
	return nil
}

// ── internals ────────────────────────────────────────────────────────────────

func (g *Gateway) get(ctx context.Context, cfg *mentity.MonitoringIntegrationConfig, path string, lk mcontracts.Lookup, out any) error {
	reqCtx, cancel := g.withTimeout(ctx, cfg)
	defer cancel()

	u := strings.TrimRight(cfg.BaseURL, "/") + path
	q := url.Values{}
	if lk.ContactID != "" {
		q.Set("contact_id", lk.ContactID)
	}
	if lk.Document != "" {
		q.Set("document", lk.Document)
	}
	if lk.Phone != "" {
		q.Set("phone", lk.Phone)
	}
	if lk.ExternalID != "" {
		q.Set("external_id", lk.ExternalID)
	}
	if encoded := q.Encode(); encoded != "" {
		u += "?" + encoded
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	g.auth(req, cfg)

	resp, err := g.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("monitoring returned status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (g *Gateway) auth(req *http.Request, cfg *mentity.MonitoringIntegrationConfig) {
	if cfg.Secret == "" {
		return
	}
	if strings.EqualFold(cfg.AuthType, "apikey") {
		req.Header.Set("X-Api-Key", cfg.Secret)
		return
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Secret)
}

func (g *Gateway) withTimeout(ctx context.Context, cfg *mentity.MonitoringIntegrationConfig) (context.Context, context.CancelFunc) {
	d := time.Duration(cfg.TimeoutMs) * time.Millisecond
	if d <= 0 {
		d = 8 * time.Second
	}
	return context.WithTimeout(ctx, d)
}

func filterMetadata(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := map[string]any{}
	for k, v := range in {
		if _, ok := allowedMetadata[k]; ok {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeStatus(s mcontracts.CustomerStatus) mcontracts.CustomerStatus {
	switch s {
	case mcontracts.StatusOnline, mcontracts.StatusOffline:
		return s
	default:
		return mcontracts.StatusUnknown
	}
}

func normalizeSeverity(s mcontracts.Severity) mcontracts.Severity {
	switch s {
	case mcontracts.SeverityWarning, mcontracts.SeverityCritical:
		return s
	default:
		return mcontracts.SeverityNormal
	}
}

var _ mcontracts.Gateway = (*Gateway)(nil)

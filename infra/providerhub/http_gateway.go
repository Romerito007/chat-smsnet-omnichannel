// Package providerhub is the HTTP client for the tenant's standardized provider
// API. It talks ONLY to that API (never to IXC/SGP/MK/Voalle directly), performs
// no caching and persists nothing. The standardized API already returns
// normalized shapes, so responses decode straight into the domain DTOs.
package providerhub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	phcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/contracts"
	phentity "github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/entity"
	infrahttp "github.com/romerito007/chat-smsnet-omnichannel/infra/http_client"
)

// Gateway is the HTTP implementation of the providerhub gateway.
type Gateway struct {
	client *http.Client
}

// NewGateway builds the gateway.
func NewGateway() *Gateway {
	return &Gateway{client: infrahttp.New(15 * time.Second)}
}

func (g *Gateway) GetCustomerProfile(ctx context.Context, cfg *phentity.ProviderIntegrationConfig, lk phcontracts.Lookup) (phcontracts.CustomerProfile, error) {
	var out phcontracts.CustomerProfile
	err := g.get(ctx, cfg, "/customers/profile", lk, &out)
	return out, err
}

func (g *Gateway) GetContracts(ctx context.Context, cfg *phentity.ProviderIntegrationConfig, lk phcontracts.Lookup) ([]phcontracts.Contract, error) {
	var out []phcontracts.Contract
	err := g.get(ctx, cfg, "/customers/contracts", lk, &out)
	return out, err
}

func (g *Gateway) GetFinancialStatus(ctx context.Context, cfg *phentity.ProviderIntegrationConfig, lk phcontracts.Lookup) (phcontracts.FinancialStatus, error) {
	var out phcontracts.FinancialStatus
	err := g.get(ctx, cfg, "/customers/financial", lk, &out)
	return out, err
}

func (g *Gateway) GetConnectionStatus(ctx context.Context, cfg *phentity.ProviderIntegrationConfig, lk phcontracts.Lookup) (phcontracts.ConnectionStatus, error) {
	var out phcontracts.ConnectionStatus
	err := g.get(ctx, cfg, "/customers/connection", lk, &out)
	return out, err
}

func (g *Gateway) GetTickets(ctx context.Context, cfg *phentity.ProviderIntegrationConfig, lk phcontracts.Lookup) ([]phcontracts.Ticket, error) {
	var out []phcontracts.Ticket
	err := g.get(ctx, cfg, "/customers/tickets", lk, &out)
	return out, err
}

func (g *Gateway) OpenTicket(ctx context.Context, cfg *phentity.ProviderIntegrationConfig, lk phcontracts.Lookup, input phcontracts.OpenTicketInput) (phcontracts.Ticket, error) {
	body := map[string]any{
		"contact_id":  lk.ContactID,
		"document":    lk.Document,
		"phone":       lk.Phone,
		"external_id": lk.ExternalID,
		"subject":     input.Subject,
		"description": input.Description,
		"priority":    input.Priority,
	}
	var out phcontracts.Ticket
	err := g.post(ctx, cfg, "/customers/tickets", body, &out)
	return out, err
}

// Ping verifies connectivity (config test).
func (g *Gateway) Ping(ctx context.Context, cfg *phentity.ProviderIntegrationConfig) error {
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
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("provider returned status %d", resp.StatusCode)
	}
	return nil
}

// ── internals ────────────────────────────────────────────────────────────────

func (g *Gateway) get(ctx context.Context, cfg *phentity.ProviderIntegrationConfig, path string, lk phcontracts.Lookup, out any) error {
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
	return g.do(req, out)
}

func (g *Gateway) post(ctx context.Context, cfg *phentity.ProviderIntegrationConfig, path string, body any, out any) error {
	reqCtx, cancel := g.withTimeout(ctx, cfg)
	defer cancel()

	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, strings.TrimRight(cfg.BaseURL, "/")+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	g.auth(req, cfg)
	return g.do(req, out)
}

func (g *Gateway) do(req *http.Request, out any) error {
	resp, err := g.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("provider returned status %d", resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (g *Gateway) auth(req *http.Request, cfg *phentity.ProviderIntegrationConfig) {
	if cfg.Secret == "" {
		return
	}
	if strings.EqualFold(cfg.AuthType, "apikey") {
		req.Header.Set("X-Api-Key", cfg.Secret)
		return
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Secret)
}

func (g *Gateway) withTimeout(ctx context.Context, cfg *phentity.ProviderIntegrationConfig) (context.Context, context.CancelFunc) {
	d := time.Duration(cfg.TimeoutMs) * time.Millisecond
	if d <= 0 {
		d = 8 * time.Second
	}
	return context.WithTimeout(ctx, d)
}

var _ phcontracts.Gateway = (*Gateway)(nil)

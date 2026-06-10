// Package service holds the providerhub business logic: config management and
// the on-demand query gateway. It never persists external payloads.
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

const defaultTimeoutMs = 8000

// TestResult is the outcome of POST /v1/providerhub/config/test.
type TestResult struct {
	OK        bool   `json:"ok"`
	LatencyMs int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

// ConfigService manages the provider integration config.
type ConfigService struct {
	repo    repository.ConfigRepository
	logs    repository.QueryLogRepository
	gateway contracts.Gateway
	clock   shared.Clock
}

// NewConfigService builds the service.
func NewConfigService(repo repository.ConfigRepository, logs repository.QueryLogRepository, gateway contracts.Gateway, clock shared.Clock) *ConfigService {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &ConfigService{repo: repo, logs: logs, gateway: gateway, clock: clock}
}

// Create registers the config, generating a secret when none is supplied.
func (s *ConfigService) Create(ctx context.Context, cmd contracts.CreateConfig) (*entity.ProviderIntegrationConfig, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cmd.BaseURL) == "" {
		return nil, apperror.Validation("base_url is required").WithDetails(map[string]any{"base_url": "is required"})
	}
	secret := cmd.Secret
	if secret == "" {
		secret = randomToken(32)
	}
	timeout := cmd.TimeoutMs
	if timeout <= 0 {
		timeout = defaultTimeoutMs
	}
	now := s.clock.Now()
	cfg := &entity.ProviderIntegrationConfig{
		ID:        shared.NewID(),
		TenantID:  tenantID,
		Name:      strings.TrimSpace(cmd.Name),
		BaseURL:   strings.TrimSpace(cmd.BaseURL),
		AuthType:  cmd.AuthType,
		Secret:    secret,
		Enabled:   true,
		TimeoutMs: timeout,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Update applies the non-nil fields of cmd to the enabled config (resolved by
// FindEnabled) — providerhub uses a single active config per tenant.
func (s *ConfigService) Update(ctx context.Context, cmd contracts.UpdateConfig) (*entity.ProviderIntegrationConfig, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	cfg, err := s.repo.FindEnabled(ctx)
	if err != nil {
		return nil, err
	}
	if cmd.Name != nil {
		cfg.Name = strings.TrimSpace(*cmd.Name)
	}
	if cmd.BaseURL != nil {
		cfg.BaseURL = strings.TrimSpace(*cmd.BaseURL)
	}
	if cmd.AuthType != nil {
		cfg.AuthType = *cmd.AuthType
	}
	if cmd.Secret != nil {
		cfg.Secret = *cmd.Secret
	}
	if cmd.Enabled != nil {
		cfg.Enabled = *cmd.Enabled
	}
	if cmd.TimeoutMs != nil && *cmd.TimeoutMs > 0 {
		cfg.TimeoutMs = *cmd.TimeoutMs
	}
	cfg.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Current returns the tenant's enabled config.
func (s *ConfigService) Current(ctx context.Context) (*entity.ProviderIntegrationConfig, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.FindEnabled(ctx)
}

// Test pings the provider API and records a minimal query log.
func (s *ConfigService) Test(ctx context.Context) (TestResult, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return TestResult{}, err
	}
	cfg, err := s.repo.FindEnabled(ctx)
	if err != nil {
		return TestResult{}, err
	}

	start := s.clock.Now()
	perr := s.gateway.Ping(ctx, cfg)
	latency := s.clock.Now().Sub(start).Milliseconds()

	status := entity.StatusSuccess
	summary := ""
	if perr != nil {
		status = entity.StatusError
		summary = summarize(perr)
	}
	s.writeLog(ctx, tenantID, "", "", entity.QueryTest, status, latency, summary)

	if perr != nil {
		return TestResult{OK: false, LatencyMs: latency, Error: "could not reach the provider API"}, nil
	}
	return TestResult{OK: true, LatencyMs: latency}, nil
}

// writeLog persists a minimal query-log entry (no payload). Best-effort.
func (s *ConfigService) writeLog(ctx context.Context, tenantID, contactID, conversationID string, qtype entity.QueryType, status entity.QueryStatus, latencyMs int64, summary string) {
	userID := ""
	if ac, ok := authz.FromContext(ctx); ok {
		userID = ac.UserID
	}
	_ = s.logs.Create(ctx, &entity.ProviderQueryLog{
		ID:             shared.NewID(),
		TenantID:       tenantID,
		UserID:         userID,
		ContactID:      contactID,
		ConversationID: conversationID,
		QueryType:      qtype,
		Status:         status,
		LatencyMs:      latencyMs,
		ErrorSummary:   summary,
		CreatedAt:      s.clock.Now(),
	})
}

// summarize trims an error to a short, body-free summary.
func summarize(err error) string {
	msg := err.Error()
	if len(msg) > 200 {
		msg = msg[:200]
	}
	return msg
}

func randomToken(n int) string {
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

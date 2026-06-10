// Package service holds the monitoring business logic: config management and the
// on-demand query gateway. It never persists external payloads.
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/monitoring/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/monitoring/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/monitoring/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

const defaultTimeoutMs = 8000

// TestResult is the outcome of POST /v1/monitoring/config/test.
type TestResult struct {
	OK        bool   `json:"ok"`
	LatencyMs int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

// ConfigService manages the monitoring integration config.
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

// Current returns the tenant's enabled config.
func (s *ConfigService) Current(ctx context.Context) (*entity.MonitoringIntegrationConfig, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.FindEnabled(ctx)
}

// Save upserts the config (PATCH semantics): it updates the enabled config or
// creates one on first configuration.
func (s *ConfigService) Save(ctx context.Context, cmd contracts.SaveConfig) (*entity.MonitoringIntegrationConfig, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()

	cfg, err := s.repo.FindEnabled(ctx)
	if err != nil {
		if apperror.From(err).Code != apperror.CodeNotFound {
			return nil, err
		}
		// First-time configuration: base_url is required.
		if cmd.BaseURL == nil || strings.TrimSpace(*cmd.BaseURL) == "" {
			return nil, apperror.Validation("base_url is required").WithDetails(map[string]any{"base_url": "is required"})
		}
		secret := ""
		if cmd.Secret != nil {
			secret = *cmd.Secret
		}
		if secret == "" {
			secret = randomToken(32)
		}
		timeout := defaultTimeoutMs
		if cmd.TimeoutMs != nil && *cmd.TimeoutMs > 0 {
			timeout = *cmd.TimeoutMs
		}
		cfg = &entity.MonitoringIntegrationConfig{
			ID:        shared.NewID(),
			TenantID:  tenantID,
			Enabled:   true,
			TimeoutMs: timeout,
			Secret:    secret,
			CreatedAt: now,
			UpdatedAt: now,
		}
		applySave(cfg, cmd)
		if err := s.repo.Create(ctx, cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	}

	applySave(cfg, cmd)
	cfg.UpdatedAt = now
	if err := s.repo.Update(ctx, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func applySave(cfg *entity.MonitoringIntegrationConfig, cmd contracts.SaveConfig) {
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
}

// Test pings the monitoring API and records a minimal query log.
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
		return TestResult{OK: false, LatencyMs: latency, Error: "could not reach the monitoring system"}, nil
	}
	return TestResult{OK: true, LatencyMs: latency}, nil
}

func (s *ConfigService) writeLog(ctx context.Context, tenantID, contactID, conversationID string, qtype entity.QueryType, status entity.QueryStatus, latencyMs int64, summary string) {
	userID := ""
	if ac, ok := authz.FromContext(ctx); ok {
		userID = ac.UserID
	}
	_ = s.logs.Create(ctx, &entity.MonitoringQueryLog{
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

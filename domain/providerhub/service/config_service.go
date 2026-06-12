// Package service holds the providerhub business logic: config management and
// the on-demand smsnet-integrations gateway. It never persists external payloads.
package service

import (
	"context"
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

// ConfigService manages the smsnet-integrations config (one active per tenant).
type ConfigService struct {
	repo    repository.ConfigRepository
	logs    repository.QueryLogRepository
	gateway contracts.Gateway
	clock   shared.Clock
	envHost string
	envKey  string
}

// NewConfigService builds the service.
func NewConfigService(repo repository.ConfigRepository, logs repository.QueryLogRepository, gateway contracts.Gateway, clock shared.Clock) *ConfigService {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &ConfigService{repo: repo, logs: logs, gateway: gateway, clock: clock}
}

// SetEnvDefault wires the env-default gateway host/key used when a tenant has no
// DB config. Optional; when unset only tenant configs resolve.
func (s *ConfigService) SetEnvDefault(host, key string) {
	s.envHost, s.envKey = host, key
}

// Resolved returns the effective config and its source ("tenant"|"env"|"none").
func (s *ConfigService) Resolved(ctx context.Context) (*entity.ProviderIntegrationConfig, string, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, "", err
	}
	return resolveConfig(ctx, s.repo, s.envHost, s.envKey)
}

// resolveConfig returns the effective provider config and its source: the tenant's
// DB config wins; otherwise the env-default gateway (host+key); a nil config with
// source "none" means unconfigured. The env key is never persisted nor returned.
func resolveConfig(ctx context.Context, repo repository.ConfigRepository, envHost, envKey string) (*entity.ProviderIntegrationConfig, string, error) {
	cfg, err := repo.FindEnabled(ctx)
	if err == nil {
		return cfg, "tenant", nil
	}
	if apperror.From(err).Code != apperror.CodeNotFound {
		return nil, "", err
	}
	if envHost == "" {
		return nil, "none", nil
	}
	tenantID, _ := shared.TenantFrom(ctx)
	return &entity.ProviderIntegrationConfig{
		TenantID: tenantID, Name: "env-default", SMSNetBaseURL: envHost, SMSNetAPIKey: envKey,
		Enabled: true, TimeoutMs: defaultTimeoutMs,
	}, "env", nil
}

// Create registers the config.
func (s *ConfigService) Create(ctx context.Context, cmd contracts.CreateConfig) (*entity.ProviderIntegrationConfig, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	v := map[string]any{}
	if strings.TrimSpace(cmd.SMSNetBaseURL) == "" {
		v["smsnet_base_url"] = "is required"
	}
	ispType := strings.TrimSpace(cmd.ISPType)
	if ispType == "" {
		v["isp_type"] = "is required"
	} else if !entity.IsKnownISPType(ispType) {
		v["isp_type"] = "unknown isp_type; expected one of " + strings.Join(entity.KnownISPTypes, ", ")
	}
	if len(v) > 0 {
		return nil, apperror.Validation("invalid provider config").WithDetails(v)
	}

	timeout := cmd.TimeoutMs
	if timeout <= 0 {
		timeout = defaultTimeoutMs
	}
	now := s.clock.Now()
	cfg := &entity.ProviderIntegrationConfig{
		ID:             shared.NewID(),
		TenantID:       tenantID,
		Name:           strings.TrimSpace(cmd.Name),
		SMSNetBaseURL:  strings.TrimSpace(cmd.SMSNetBaseURL),
		SMSNetAPIKey:   cmd.SMSNetAPIKey,
		ISPType:        ispType,
		ISPCredentials: cmd.ISPCredentials,
		BotID:          strings.TrimSpace(cmd.BotID),
		Options: entity.Options{
			UsaPegarFaturaAtrasada:      cmd.UsaPegarFaturaAtrasada,
			UsaExtrairLinhaDigitavelPDF: cmd.UsaExtrairLinhaDigitavelPDF,
			DadosPlanos:                 cmd.DadosPlanos,
			DadosEmpresa:                cmd.DadosEmpresa,
		},
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

// Update applies the non-nil fields of cmd to the enabled config.
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
	if cmd.SMSNetBaseURL != nil {
		cfg.SMSNetBaseURL = strings.TrimSpace(*cmd.SMSNetBaseURL)
	}
	if cmd.SMSNetAPIKey != nil {
		cfg.SMSNetAPIKey = *cmd.SMSNetAPIKey
	}
	if cmd.ISPType != nil {
		t := strings.TrimSpace(*cmd.ISPType)
		if !entity.IsKnownISPType(t) {
			return nil, apperror.Validation("unknown isp_type").
				WithDetails(map[string]any{"isp_type": "expected one of " + strings.Join(entity.KnownISPTypes, ", ")})
		}
		cfg.ISPType = t
	}
	if cmd.ISPCredentials != nil {
		cfg.ISPCredentials = *cmd.ISPCredentials
	}
	if cmd.BotID != nil {
		cfg.BotID = strings.TrimSpace(*cmd.BotID)
	}
	if cmd.Enabled != nil {
		cfg.Enabled = *cmd.Enabled
	}
	if cmd.TimeoutMs != nil && *cmd.TimeoutMs > 0 {
		cfg.TimeoutMs = *cmd.TimeoutMs
	}
	if cmd.UsaPegarFaturaAtrasada != nil {
		cfg.Options.UsaPegarFaturaAtrasada = *cmd.UsaPegarFaturaAtrasada
	}
	if cmd.UsaExtrairLinhaDigitavelPDF != nil {
		cfg.Options.UsaExtrairLinhaDigitavelPDF = *cmd.UsaExtrairLinhaDigitavelPDF
	}
	if cmd.DadosPlanos != nil {
		cfg.Options.DadosPlanos = *cmd.DadosPlanos
	}
	if cmd.DadosEmpresa != nil {
		cfg.Options.DadosEmpresa = *cmd.DadosEmpresa
	}
	cfg.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Test pings the smsnet-integrations API and records a minimal query log.
func (s *ConfigService) Test(ctx context.Context) (TestResult, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return TestResult{}, err
	}
	cfg, _, err := resolveConfig(ctx, s.repo, s.envHost, s.envKey)
	if err != nil {
		return TestResult{}, err
	}
	if cfg == nil {
		return TestResult{}, apperror.Integration("provider integration is not configured")
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

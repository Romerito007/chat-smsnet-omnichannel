// Package service holds the copilot business logic: per-tenant config and the
// policy-respecting inference use cases.
package service

import (
	"context"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// Defaults for a freshly created config. Data-access flags default to false
// (privacy-safe): a tenant must explicitly opt in to share customer/financial/
// monitoring data with the provider.
const (
	defaultProvider    = entity.ProviderOpenAI
	defaultModel       = "gpt-4o-mini"
	defaultTemperature = 0.7
	defaultMaxTokens   = 512
)

// ConfigService manages the per-tenant copilot configuration.
type ConfigService struct {
	repo    repository.ConfigRepository
	clock   shared.Clock
	auditor shared.Auditor
}

// NewConfigService builds the service.
func NewConfigService(repo repository.ConfigRepository, clock shared.Clock) *ConfigService {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &ConfigService{repo: repo, clock: clock, auditor: shared.NoopAuditor{}}
}

// SetAuditor wires the audit trail. Optional: when unset, AI config changes are
// not audited.
func (s *ConfigService) SetAuditor(a shared.Auditor) {
	if a != nil {
		s.auditor = a
	}
}

// auditSaved records an AI configuration change.
func (s *ConfigService) auditSaved(ctx context.Context, cfg *entity.AIConfig) {
	_ = s.auditor.Record(ctx, shared.AuditEntry{
		Action: "ai.config.updated", ResourceType: "ai_config", ResourceID: cfg.TenantID,
		Data: map[string]any{
			"provider":            string(cfg.Provider),
			"model":               cfg.Model,
			"allow_customer_data": cfg.AllowCustomerData,
		},
	})
}

// Current returns the tenant's config, creating a default one on first access
// (the tenant then sets its provider API key before the copilot can run).
func (s *ConfigService) Current(ctx context.Context) (*entity.AIConfig, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	cfg, err := s.repo.FindByTenant(ctx)
	if err != nil {
		if apperror.From(err).Code != apperror.CodeNotFound {
			return nil, err
		}
		return s.defaultConfig(tenantID), nil
	}
	return cfg, nil
}

// Save upserts the tenant's config (PATCH semantics).
func (s *ConfigService) Save(ctx context.Context, cmd contracts.SaveConfig) (*entity.AIConfig, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()

	cfg, err := s.repo.FindByTenant(ctx)
	if err != nil {
		if apperror.From(err).Code != apperror.CodeNotFound {
			return nil, err
		}
		cfg = s.defaultConfig(tenantID)
		if aerr := applySave(cfg, cmd); aerr != nil {
			return nil, aerr
		}
		cfg.CreatedAt = now
		cfg.UpdatedAt = now
		if err := s.repo.Create(ctx, cfg); err != nil {
			return nil, err
		}
		s.auditSaved(ctx, cfg)
		return cfg, nil
	}

	if aerr := applySave(cfg, cmd); aerr != nil {
		return nil, aerr
	}
	cfg.UpdatedAt = now
	if err := s.repo.Update(ctx, cfg); err != nil {
		return nil, err
	}
	s.auditSaved(ctx, cfg)
	return cfg, nil
}

func (s *ConfigService) defaultConfig(tenantID string) *entity.AIConfig {
	now := s.clock.Now()
	return &entity.AIConfig{
		ID:          shared.NewID(),
		TenantID:    tenantID,
		Provider:    defaultProvider,
		Model:       defaultModel,
		Temperature: defaultTemperature,
		MaxTokens:   defaultMaxTokens,
		Enabled:     true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func applySave(cfg *entity.AIConfig, cmd contracts.SaveConfig) error {
	if cmd.Provider != nil {
		p := entity.Provider(strings.TrimSpace(*cmd.Provider))
		if !entity.IsValidProvider(p) {
			return apperror.Validation("invalid provider").WithDetails(map[string]any{"provider": "unsupported"})
		}
		cfg.Provider = p
	}
	if cmd.Model != nil {
		cfg.Model = strings.TrimSpace(*cmd.Model)
	}
	if cmd.APIKey != nil {
		// Empty string clears the key; otherwise set the new credential.
		cfg.APIKey = strings.TrimSpace(*cmd.APIKey)
	}
	if cmd.BaseURL != nil {
		cfg.BaseURL = strings.TrimSpace(*cmd.BaseURL)
	}
	if cmd.Temperature != nil {
		if *cmd.Temperature < 0 || *cmd.Temperature > 2 {
			return apperror.Validation("temperature must be between 0 and 2").WithDetails(map[string]any{"temperature": "out of range"})
		}
		cfg.Temperature = *cmd.Temperature
	}
	if cmd.MaxTokens != nil {
		if *cmd.MaxTokens <= 0 {
			return apperror.Validation("max_tokens must be positive").WithDetails(map[string]any{"max_tokens": "must be positive"})
		}
		cfg.MaxTokens = *cmd.MaxTokens
	}
	if cmd.AllowCustomerData != nil {
		cfg.AllowCustomerData = *cmd.AllowCustomerData
	}
	if cmd.AllowFinancialData != nil {
		cfg.AllowFinancialData = *cmd.AllowFinancialData
	}
	if cmd.AllowMonitoringData != nil {
		cfg.AllowMonitoringData = *cmd.AllowMonitoringData
	}
	if cmd.HumanApprovalRequired != nil {
		cfg.HumanApprovalRequired = *cmd.HumanApprovalRequired
	}
	if cmd.Enabled != nil {
		cfg.Enabled = *cmd.Enabled
	}
	return nil
}

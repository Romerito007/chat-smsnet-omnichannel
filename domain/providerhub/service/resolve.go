package service

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

const defaultTimeoutMs = 8000

// resolveConfig returns the effective provider config and its source: the tenant's
// legacy single DB config wins; otherwise the env-default gateway (host+key); a nil
// config with source "none" means unconfigured. The env key is never persisted nor
// returned.
//
// NOTE (F1): the conversation QueryService still uses this legacy single-config
// resolution. F2 replaces it with the ISP-profile resolver (explicit id > default).
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

// summarize trims an error to a short, body-free summary.
func summarize(err error) string {
	msg := err.Error()
	if len(msg) > 200 {
		msg = msg[:200]
	}
	return msg
}

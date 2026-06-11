package contracts

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/entity"
)

// Gateway is the on-demand client to the tenant's smsnet-integrations API. The
// implementation (infra/providerhub) builds the body
// { botId, <route fields>, config: { type, <isp_credentials> } } and sends the
// x-api-key header. It maps the response envelope (success/not_found/needs_input/
// fallback) to normalized DTOs or domain errors, never persisting the payload.
type Gateway interface {
	ConsultarCliente(ctx context.Context, cfg *entity.ProviderIntegrationConfig, req ConsultaClienteRequest) (ClienteResult, error)
	ListarPlanos(ctx context.Context, cfg *entity.ProviderIntegrationConfig) ([]Plano, error)
	DadosEmpresa(ctx context.Context, cfg *entity.ProviderIntegrationConfig) (Empresa, error)
	LiberarAcesso(ctx context.Context, cfg *entity.ProviderIntegrationConfig, idCliente string) (Liberacao, error)
	AbrirChamado(ctx context.Context, cfg *entity.ProviderIntegrationConfig, idCliente, subject, message string) (Chamado, error)
	// Ping verifies connectivity/credentials for the config test.
	Ping(ctx context.Context, cfg *entity.ProviderIntegrationConfig) error
}

// RateLimiter caps the per-tenant rate of outbound provider queries, protecting
// the upstream API. The implementation lives in infra/providerhub.
type RateLimiter interface {
	// Allow reports whether another query is permitted for the tenant right now.
	Allow(ctx context.Context, tenantID string) (bool, error)
}

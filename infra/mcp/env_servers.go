package mcp

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/mcp/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/mcp/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// Well-known identities for the env-default SMSNET MCP servers.
const (
	envConsultasID   = "env-smsnet-consultas"
	envOperacoesID   = "env-smsnet-operacoes"
	envConsultasName = "SMSNET_CONSULTAS"
	envOperacoesName = "SMSNET_OPERACOES"
)

// EnvServerRepository augments the DB-backed MCP server repository with the two
// env-default SMSNET servers (CONSULTAS = read, OPERACOES = write). Resolution is
// "tenant DB override → env default": a tenant server registered under the same
// name suppresses the env one (so the tenant URL wins). The env servers carry NO
// auth (they live on a private network) and their URLs are never surfaced through
// the admin server list — that path uses the inner repository directly.
type EnvServerRepository struct {
	repository.ServerRepository // inner (Mongo); Create/Update/Delete/List delegate
	consultasURL                string
	operacoesURL                string
}

// NewEnvServerRepository wraps inner with the env-default servers. URLs left empty
// disable the corresponding env server.
func NewEnvServerRepository(inner repository.ServerRepository, consultasURL, operacoesURL string) *EnvServerRepository {
	return &EnvServerRepository{ServerRepository: inner, consultasURL: consultasURL, operacoesURL: operacoesURL}
}

func (r *EnvServerRepository) envServers(tenantID string) []*entity.ServerConnection {
	var out []*entity.ServerConnection
	if r.consultasURL != "" {
		out = append(out, &entity.ServerConnection{
			ID: envConsultasID, TenantID: tenantID, Name: envConsultasName,
			Transport: entity.TransportStreamableHTTP, BaseURL: r.consultasURL,
			Kind: entity.KindRead, Enabled: true,
		})
	}
	if r.operacoesURL != "" {
		out = append(out, &entity.ServerConnection{
			ID: envOperacoesID, TenantID: tenantID, Name: envOperacoesName,
			Transport: entity.TransportStreamableHTTP, BaseURL: r.operacoesURL,
			Kind: entity.KindWrite, Enabled: true,
		})
	}
	return out
}

// ListEnabled returns the tenant's DB servers plus any env-default server whose
// name the tenant has not overridden.
func (r *EnvServerRepository) ListEnabled(ctx context.Context) ([]*entity.ServerConnection, error) {
	db, err := r.ServerRepository.ListEnabled(ctx)
	if err != nil {
		return nil, err
	}
	names := make(map[string]bool, len(db))
	for _, s := range db {
		names[s.Name] = true
	}
	tid, _ := shared.TenantFrom(ctx)
	out := append([]*entity.ServerConnection{}, db...)
	for _, e := range r.envServers(tid) {
		if !names[e.Name] {
			out = append(out, e)
		}
	}
	return out, nil
}

// FindByID resolves the synthetic env ids first (so Run/Decide can reference them),
// falling back to the inner repository for DB servers.
func (r *EnvServerRepository) FindByID(ctx context.Context, id string) (*entity.ServerConnection, error) {
	tid, _ := shared.TenantFrom(ctx)
	for _, e := range r.envServers(tid) {
		if e.ID == id {
			return e, nil
		}
	}
	return r.ServerRepository.FindByID(ctx, id)
}

var _ repository.ServerRepository = (*EnvServerRepository)(nil)

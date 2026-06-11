package entity

import "time"

// QueryType identifies a provider gateway operation against smsnet-integrations.
type QueryType string

const (
	QueryConsultarCliente QueryType = "consultar_cliente"
	QueryListarPlanos     QueryType = "listar_planos"
	QueryDadosEmpresa     QueryType = "dados_empresa"
	QueryLiberarAcesso    QueryType = "liberar_acesso"
	QueryAbrirChamado     QueryType = "abrir_chamado"
	QueryTest             QueryType = "test"
)

// QueryStatus is the outcome of a query.
type QueryStatus string

const (
	StatusSuccess    QueryStatus = "success"
	StatusNotFound   QueryStatus = "not_found"
	StatusNeedsInput QueryStatus = "needs_input"
	StatusError      QueryStatus = "error"
	StatusTimeout    QueryStatus = "timeout"
	StatusBlocked    QueryStatus = "blocked" // rate-limited
)

// ProviderQueryLog is the minimal technical record of one on-demand query. It
// deliberately stores NO response body — only metadata for auditing/diagnostics.
type ProviderQueryLog struct {
	ID             string
	TenantID       string
	UserID         string
	ContactID      string
	ConversationID string
	QueryType      QueryType
	Status         QueryStatus
	LatencyMs      int64
	ErrorSummary   string
	CreatedAt      time.Time
}

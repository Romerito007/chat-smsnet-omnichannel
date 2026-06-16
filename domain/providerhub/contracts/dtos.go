// Package contracts holds the normalized provider DTOs, the gateway port and the
// service inputs. These DTOs are returned to clients on demand and are NEVER
// persisted.
package contracts

import "context"

// ConsultaClienteRequest identifies a customer to the smsnet-integrations API.
// One of CpfCnpj/Phone/Email locates the customer; IDCliente targets a specific
// contract after a needs_input selection. ISPConfigID optionally pins the ISP
// profile to use (else the tenant default).
type ConsultaClienteRequest struct {
	CpfCnpj     string
	Phone       string
	Email       string
	IDCliente   string
	ISPConfigID string
}

// idemKeyCtx carries the idempotency key down to the gateway so side-effect calls
// (liberacao/chamado) forward an Idempotency-Key header to the smsnet-integrations
// API for upstream dedup.
type idemKeyCtx struct{}

// WithIdempotencyKey returns a context carrying the idempotency key.
func WithIdempotencyKey(ctx context.Context, key string) context.Context {
	if key == "" {
		return ctx
	}
	return context.WithValue(ctx, idemKeyCtx{}, key)
}

// IdempotencyKeyFrom returns the idempotency key from the context, or "".
func IdempotencyKeyFrom(ctx context.Context) string {
	if v, ok := ctx.Value(idemKeyCtx{}).(string); ok {
		return v
	}
	return ""
}

// ContratoOption is one selectable contract returned on a needs_input response.
type ContratoOption struct {
	IDCliente string `json:"id_cliente"`
	Label     string `json:"label"`
	Endereco  string `json:"endereco,omitempty"`
	Status    string `json:"status,omitempty"`
}

// Fatura is a normalized invoice line.
type Fatura struct {
	Valor          float64 `json:"valor"`
	Vencimento     string  `json:"vencimento,omitempty"`
	Link           string  `json:"link,omitempty"`
	LinhaDigitavel string  `json:"linha_digitavel,omitempty"`
	Pix            string  `json:"pix,omitempty"`
}

// Cliente is the normalized customer record.
type Cliente struct {
	Nome                  string   `json:"nome,omitempty"`
	CpfCnpj               string   `json:"cpfcnpj,omitempty"`
	ContratoStatusDisplay string   `json:"contrato_status_display,omitempty"`
	ValorCheckOut         float64  `json:"valor_check_out,omitempty"`
	Faturas               []Fatura `json:"faturas,omitempty"`
}

// ClienteResult is the outcome of ConsultarCliente. On a needs_input response,
// NeedsSelection is true and Options lists the contracts for the agent to pick;
// the next call carries the chosen IDCliente. On success, Cliente is set.
type ClienteResult struct {
	NeedsSelection bool             `json:"needs_selection"`
	Options        []ContratoOption `json:"options,omitempty"`
	Cliente        *Cliente         `json:"cliente,omitempty"`
}

// Plano is a normalized plan/offer.
type Plano struct {
	Nome       string  `json:"nome"`
	Valor      float64 `json:"valor,omitempty"`
	Velocidade string  `json:"velocidade,omitempty"`
	Descricao  string  `json:"descricao,omitempty"`
}

// Empresa is the normalized company/ISP profile.
type Empresa struct {
	Nome     string `json:"nome,omitempty"`
	CNPJ     string `json:"cnpj,omitempty"`
	Telefone string `json:"telefone,omitempty"`
	Email    string `json:"email,omitempty"`
	Endereco string `json:"endereco,omitempty"`
	Site     string `json:"site,omitempty"`
}

// Liberacao is the outcome of a trust-unlock (liberar acesso) action.
type Liberacao struct {
	Liberado    bool   `json:"liberado"`
	Protocolo   string `json:"protocolo,omitempty"`
	LiberadoAte string `json:"liberado_ate,omitempty"`
	Msg         string `json:"msg,omitempty"`
}

// Chamado is the outcome of opening a support ticket.
type Chamado struct {
	Protocolo string `json:"protocolo,omitempty"`
	Msg       string `json:"msg,omitempty"`
}

// CreateProfile registers a new ISP profile (multiple per tenant). The SMSNET
// gateway host/key are NOT part of a profile (they are infra/env).
type CreateProfile struct {
	Label                       string
	ISPType                     string
	Credentials                 map[string]string
	Transports                  []string // REQUIRED: subset of {http, mcp}, at least one
	EnabledActions              []string // nil → all catalog actions; else subset of them
	IsDefault                   bool
	UsaPegarFaturaAtrasada      bool
	UsaExtrairLinhaDigitavelPDF bool
	TimeoutMs                   int
	Enabled                     *bool // nil → true
}

// UpdateProfile carries optional fields; nil pointers mean "leave unchanged".
// is_default is changed via SetDefault, not here.
type UpdateProfile struct {
	Label                       *string
	ISPType                     *string
	Credentials                 *map[string]string
	Transports                  *[]string // when set, replaces the transports (still validated non-empty)
	EnabledActions              *[]string // when set, replaces the enabled actions (subset of the ISP catalog)
	UsaPegarFaturaAtrasada      *bool
	UsaExtrairLinhaDigitavelPDF *bool
	TimeoutMs                   *int
	Enabled                     *bool
}

// GatewayStatus is the GET /v1/providerhub/config result: the shared SMSNET
// gateway is infra (env) now, so this reports whether it is configured plus a
// summary of the tenant's ISP profiles.
type GatewayStatus struct {
	Source           string
	Configured       bool
	HasProfiles      bool
	DefaultProfileID string
	ProfilesCount    int
}

// TestResult is the outcome of a per-profile gateway test.
type TestResult struct {
	OK        bool   `json:"ok"`
	LatencyMs int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

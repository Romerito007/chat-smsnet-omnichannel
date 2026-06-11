// Package contracts holds the normalized provider DTOs, the gateway port and the
// service inputs. These DTOs are returned to clients on demand and are NEVER
// persisted.
package contracts

// ConsultaClienteRequest identifies a customer to the smsnet-integrations API.
// One of CpfCnpj/Phone/Email locates the customer; IDCliente targets a specific
// contract after a needs_input selection.
type ConsultaClienteRequest struct {
	CpfCnpj   string
	Phone     string
	Email     string
	IDCliente string
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

// CreateConfig registers a smsnet-integrations config.
type CreateConfig struct {
	Name                        string
	SMSNetBaseURL               string
	SMSNetAPIKey                string
	ISPType                     string
	ISPCredentials              map[string]string
	BotID                       string
	TimeoutMs                   int
	UsaPegarFaturaAtrasada      bool
	UsaExtrairLinhaDigitavelPDF bool
	DadosPlanos                 map[string]any
	DadosEmpresa                map[string]any
}

// UpdateConfig carries optional fields; nil pointers mean "leave unchanged".
type UpdateConfig struct {
	Name                        *string
	SMSNetBaseURL               *string
	SMSNetAPIKey                *string
	ISPType                     *string
	ISPCredentials              *map[string]string
	BotID                       *string
	Enabled                     *bool
	TimeoutMs                   *int
	UsaPegarFaturaAtrasada      *bool
	UsaExtrairLinhaDigitavelPDF *bool
	DadosPlanos                 *map[string]any
	DadosEmpresa                *map[string]any
}

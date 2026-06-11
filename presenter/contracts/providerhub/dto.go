// Package providerhub holds the request/response DTOs for the providerhub
// endpoints. The API key and ISP credentials are never returned (only whether
// they are set, plus the credential keys); external query payloads pass through
// the normalized domain DTOs and are never persisted.
package providerhub

import (
	"sort"
	"time"

	phcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/contracts"
	phentity "github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/entity"
)

// CreateConfigRequest is the body of POST /v1/providerhub/config.
type CreateConfigRequest struct {
	Name                        string            `json:"name"`
	SMSNetBaseURL               string            `json:"smsnet_base_url"`
	SMSNetAPIKey                string            `json:"smsnet_api_key"`
	ISPType                     string            `json:"isp_type"`
	ISPCredentials              map[string]string `json:"isp_credentials"`
	BotID                       string            `json:"bot_id"`
	TimeoutMs                   int               `json:"timeout_ms"`
	UsaPegarFaturaAtrasada      bool              `json:"usa_pegar_fatura_atrasada"`
	UsaExtrairLinhaDigitavelPDF bool              `json:"usa_extrair_linha_digitavel_pdf"`
	DadosPlanos                 map[string]any    `json:"dados_planos"`
	DadosEmpresa                map[string]any    `json:"dados_empresa"`
}

// ToCommand maps to the service command.
func (r CreateConfigRequest) ToCommand() phcontracts.CreateConfig {
	return phcontracts.CreateConfig{
		Name:                        r.Name,
		SMSNetBaseURL:               r.SMSNetBaseURL,
		SMSNetAPIKey:                r.SMSNetAPIKey,
		ISPType:                     r.ISPType,
		ISPCredentials:              r.ISPCredentials,
		BotID:                       r.BotID,
		TimeoutMs:                   r.TimeoutMs,
		UsaPegarFaturaAtrasada:      r.UsaPegarFaturaAtrasada,
		UsaExtrairLinhaDigitavelPDF: r.UsaExtrairLinhaDigitavelPDF,
		DadosPlanos:                 r.DadosPlanos,
		DadosEmpresa:                r.DadosEmpresa,
	}
}

// UpdateConfigRequest is the body of PATCH /v1/providerhub/config.
type UpdateConfigRequest struct {
	Name                        *string            `json:"name"`
	SMSNetBaseURL               *string            `json:"smsnet_base_url"`
	SMSNetAPIKey                *string            `json:"smsnet_api_key"`
	ISPType                     *string            `json:"isp_type"`
	ISPCredentials              *map[string]string `json:"isp_credentials"`
	BotID                       *string            `json:"bot_id"`
	Enabled                     *bool              `json:"enabled"`
	TimeoutMs                   *int               `json:"timeout_ms"`
	UsaPegarFaturaAtrasada      *bool              `json:"usa_pegar_fatura_atrasada"`
	UsaExtrairLinhaDigitavelPDF *bool              `json:"usa_extrair_linha_digitavel_pdf"`
	DadosPlanos                 *map[string]any    `json:"dados_planos"`
	DadosEmpresa                *map[string]any    `json:"dados_empresa"`
}

// ToCommand maps to the service command.
func (r UpdateConfigRequest) ToCommand() phcontracts.UpdateConfig {
	return phcontracts.UpdateConfig{
		Name:                        r.Name,
		SMSNetBaseURL:               r.SMSNetBaseURL,
		SMSNetAPIKey:                r.SMSNetAPIKey,
		ISPType:                     r.ISPType,
		ISPCredentials:              r.ISPCredentials,
		BotID:                       r.BotID,
		Enabled:                     r.Enabled,
		TimeoutMs:                   r.TimeoutMs,
		UsaPegarFaturaAtrasada:      r.UsaPegarFaturaAtrasada,
		UsaExtrairLinhaDigitavelPDF: r.UsaExtrairLinhaDigitavelPDF,
		DadosPlanos:                 r.DadosPlanos,
		DadosEmpresa:                r.DadosEmpresa,
	}
}

// ConfigResponse is the public representation. Secrets are masked: only whether
// the API key is set and the credential KEYS (never the values) are exposed.
type ConfigResponse struct {
	ID                          string    `json:"id"`
	TenantID                    string    `json:"tenant_id"`
	Name                        string    `json:"name,omitempty"`
	SMSNetBaseURL               string    `json:"smsnet_base_url"`
	ISPType                     string    `json:"isp_type"`
	BotID                       string    `json:"bot_id,omitempty"`
	HasAPIKey                   bool      `json:"has_api_key"`
	ISPCredentialKeys           []string  `json:"isp_credential_keys,omitempty"`
	UsaPegarFaturaAtrasada      bool      `json:"usa_pegar_fatura_atrasada"`
	UsaExtrairLinhaDigitavelPDF bool      `json:"usa_extrair_linha_digitavel_pdf"`
	Enabled                     bool      `json:"enabled"`
	TimeoutMs                   int       `json:"timeout_ms"`
	CreatedAt                   time.Time `json:"created_at"`
	UpdatedAt                   time.Time `json:"updated_at"`
}

// NewConfigResponse maps a config entity, masking secrets.
func NewConfigResponse(c *phentity.ProviderIntegrationConfig) ConfigResponse {
	keys := make([]string, 0, len(c.ISPCredentials))
	for k := range c.ISPCredentials {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return ConfigResponse{
		ID:                          c.ID,
		TenantID:                    c.TenantID,
		Name:                        c.Name,
		SMSNetBaseURL:               c.SMSNetBaseURL,
		ISPType:                     c.ISPType,
		BotID:                       c.BotID,
		HasAPIKey:                   c.SMSNetAPIKey != "",
		ISPCredentialKeys:           keys,
		UsaPegarFaturaAtrasada:      c.Options.UsaPegarFaturaAtrasada,
		UsaExtrairLinhaDigitavelPDF: c.Options.UsaExtrairLinhaDigitavelPDF,
		Enabled:                     c.Enabled,
		TimeoutMs:                   c.TimeoutMs,
		CreatedAt:                   c.CreatedAt,
		UpdatedAt:                   c.UpdatedAt,
	}
}

// LiberacaoRequest is the body of POST /v1/conversations/{id}/external/liberacao.
type LiberacaoRequest struct {
	IDCliente string `json:"id_cliente"`
}

// ChamadoRequest is the body of POST /v1/conversations/{id}/external/chamado.
type ChamadoRequest struct {
	IDCliente string `json:"id_cliente"`
	Subject   string `json:"subject"`
	Message   string `json:"message"`
}

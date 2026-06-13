package models

import "time"

// ProviderIntegrationConfig is the BSON document for the smsnet-integrations
// config. The API key and the ISP credentials map are stored encrypted.
type ProviderIntegrationConfig struct {
	Base                 `bson:",inline"`
	Name                 string                `bson:"name,omitempty"`
	SMSNetBaseURL        string                `bson:"smsnet_base_url"`
	EncryptedAPIKey      string                `bson:"encrypted_api_key,omitempty"`
	ISPType              string                `bson:"isp_type"`
	EncryptedCredentials string                `bson:"encrypted_credentials,omitempty"` // encrypted JSON of the credentials map
	BotID                string                `bson:"bot_id,omitempty"`
	Options              ProviderConfigOptions `bson:"options"`
	Enabled              bool                  `bson:"enabled"`
	TimeoutMs            int                   `bson:"timeout_ms"`
}

// ProviderConfigOptions are the non-secret per-tenant toggles and fixed data.
type ProviderConfigOptions struct {
	UsaPegarFaturaAtrasada      bool           `bson:"usa_pegar_fatura_atrasada"`
	UsaExtrairLinhaDigitavelPDF bool           `bson:"usa_extrair_linha_digitavel_pdf"`
	DadosPlanos                 map[string]any `bson:"dados_planos,omitempty"`
	DadosEmpresa                map[string]any `bson:"dados_empresa,omitempty"`
}

// ISPProfile is the BSON document for one per-tenant ISP profile. The credentials
// map is stored encrypted; the SMSNET gateway host/key are NOT stored here (they
// are infra/env). At most one profile per tenant has is_default=true (partial
// unique index).
type ISPProfile struct {
	Base                 `bson:",inline"`
	Label                string            `bson:"label"`
	ISPType              string            `bson:"isp_type"`
	EncryptedCredentials string            `bson:"encrypted_credentials,omitempty"` // encrypted JSON of the credentials map
	IsDefault            bool              `bson:"is_default"`
	Options              ISPProfileOptions `bson:"options"`
	TimeoutMs            int               `bson:"timeout_ms"`
	Enabled              bool              `bson:"enabled"`
}

// ISPProfileOptions are the non-secret per-profile toggles.
type ISPProfileOptions struct {
	UsaPegarFaturaAtrasada      bool `bson:"usa_pegar_fatura_atrasada"`
	UsaExtrairLinhaDigitavelPDF bool `bson:"usa_extrair_linha_digitavel_pdf"`
}

// ProviderQueryLog is the BSON document for the minimal technical query log.
// It deliberately stores no response body.
type ProviderQueryLog struct {
	ID             string    `bson:"_id"`
	TenantID       string    `bson:"tenant_id"`
	UserID         string    `bson:"user_id,omitempty"`
	ContactID      string    `bson:"contact_id,omitempty"`
	ConversationID string    `bson:"conversation_id,omitempty"`
	QueryType      string    `bson:"query_type"`
	Status         string    `bson:"status"`
	LatencyMs      int64     `bson:"latency_ms"`
	ErrorSummary   string    `bson:"error_summary,omitempty"`
	CreatedAt      time.Time `bson:"created_at"`
}

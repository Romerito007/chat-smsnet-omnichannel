package models

import "time"

// ISPProfile is the BSON document for one per-tenant ISP profile. The credentials
// map is stored encrypted; the SMSNET gateway host/key are NOT stored here (they
// are infra/env). At most one profile per tenant has is_default=true (partial
// unique index).
type ISPProfile struct {
	Base                 `bson:",inline"`
	Label                string            `bson:"label"`
	ISPType              string            `bson:"isp_type"`
	EncryptedCredentials string            `bson:"encrypted_credentials,omitempty"` // encrypted JSON of the credentials map
	Transports           []string          `bson:"transports"`                      // enabled SMSNET surfaces (http/mcp)
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

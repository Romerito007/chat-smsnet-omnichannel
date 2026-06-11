// Package entity holds the providerhub aggregates: the integration config (which
// points at the tenant's smsnet-integrations API) and the minimal technical query
// log. No external provider payloads are persisted.
package entity

import "time"

// ISP type slugs supported by the smsnet-integrations API. Sent as config.type.
const (
	ISPHubsoft = "hubsoft"
	ISPSGPNet  = "sgpnet"
	ISPIXCSoft = "ixcsoft"
	ISPMKAuth  = "mkauth"
	ISPVoalle  = "voalle"
	ISPSGP     = "sgp"
)

// KnownISPTypes is the closed set of accepted isp_type slugs.
var KnownISPTypes = []string{ISPHubsoft, ISPSGPNet, ISPIXCSoft, ISPMKAuth, ISPVoalle, ISPSGP}

// IsKnownISPType reports whether t is an accepted isp_type slug.
func IsKnownISPType(t string) bool {
	for _, s := range KnownISPTypes {
		if s == t {
			return true
		}
	}
	return false
}

// Options are per-tenant feature toggles and fixed data forwarded to the API.
type Options struct {
	// UsaPegarFaturaAtrasada asks the API to include overdue invoices.
	UsaPegarFaturaAtrasada bool
	// UsaExtrairLinhaDigitavelPDF asks the API to extract the boleto line from PDF.
	UsaExtrairLinhaDigitavelPDF bool
	// DadosPlanos / DadosEmpresa are optional fixed datasets the tenant configures
	// (returned by ListarPlanos / DadosEmpresa without hitting the ISP).
	DadosPlanos  map[string]any
	DadosEmpresa map[string]any
}

// ProviderIntegrationConfig points at the tenant's smsnet-integrations API and
// carries the ISP-specific credentials. SMSNetAPIKey and ISPCredentials are
// encrypted at rest and never returned.
type ProviderIntegrationConfig struct {
	ID             string
	TenantID       string
	Name           string
	SMSNetBaseURL  string            // smsnet_base_url
	SMSNetAPIKey   string            // smsnet_api_key (x-api-key header), encrypted
	ISPType        string            // isp_type slug
	ISPCredentials map[string]string // isp_credentials (encrypted): hubsoft_host, hubsoft_client_id, ...
	Options        Options
	BotID          string // bot_id sent as botId (may default to the tenant id)
	Enabled        bool
	TimeoutMs      int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ResolveBotID returns the configured bot id, defaulting to the tenant id.
func (c *ProviderIntegrationConfig) ResolveBotID() string {
	if c.BotID != "" {
		return c.BotID
	}
	return c.TenantID
}

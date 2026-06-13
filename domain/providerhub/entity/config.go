// Package entity holds the providerhub aggregates: the integration config (which
// points at the tenant's smsnet-integrations API) and the minimal technical query
// log. No external provider payloads are persisted.
package entity

import (
	"strings"
	"time"
)

// A few well-known slugs kept as named constants for the gateway/tests.
const (
	ISPHubsoft     = "hubsoft"
	ISPSGPNet      = "sgpnet"
	ISPIXCSoft     = "ixcsoft"
	ISPMKAuth      = "mkauth"
	ISPMKSolutions = "mksolutions"
	ISPWHMCS       = "whmcs"
	// Legacy slugs accepted for backward compatibility with already-stored
	// configs (they predate the SMSNET 19-ISP catalog). Not in the catalog.
	ISPVoalle = "voalle"
	ISPSGP    = "sgp"
)

// ISPAction is a provider action the smsnet-integrations API exposes.
type ISPAction string

const (
	ActionCliente   ISPAction = "cliente"
	ActionPlanos    ISPAction = "planos"
	ActionEmpresa   ISPAction = "empresa"
	ActionLiberacao ISPAction = "liberacao"
	ActionChamado   ISPAction = "chamado"
)

// ISPCredentialField describes one credential input the UI must render for an
// ISP. Key is the exact config.<key> the SMSNET API expects; Secret marks a
// value that must be a masked input and is never echoed back.
type ISPCredentialField struct {
	Key    string
	Label  string
	Secret bool
}

// ISPDescriptor is the catalog entry for one supported ISP: its slug, a label,
// the required credential fields and the actions it supports — so the front
// renders the right inputs and shows/hides actions without hard-coding any of it.
type ISPDescriptor struct {
	Slug        string
	Label       string
	Credentials []ISPCredentialField
	Actions     []ISPAction
	SearchBy    []string // "cpfcnpj" | "phone" | "email"
}

// ISPCatalogVersion versions the catalog so the front can cache it.
const ISPCatalogVersion = "2026-06-13"

// secretCredKeyParts marks a credential field as secret when its key contains any
// of these tokens (host/email/username/identifier/client_id are not secrets).
var secretCredKeyParts = []string{"token", "secret", "password", "senha", "appkey", "rtoken"}

// creds builds the credential fields for an ISP, auto-labeling each and flagging
// secrets by key.
func creds(keys ...string) []ISPCredentialField {
	out := make([]ISPCredentialField, len(keys))
	for i, k := range keys {
		out[i] = ISPCredentialField{Key: k, Label: credLabel(k), Secret: isSecretCredKey(k)}
	}
	return out
}

func isSecretCredKey(key string) bool {
	for _, p := range secretCredKeyParts {
		if strings.Contains(key, p) {
			return true
		}
	}
	return false
}

// credLabel turns "hubsoft_client_secret" into "Client secret".
func credLabel(key string) string {
	name := key
	if parts := strings.SplitN(key, "_", 2); len(parts) == 2 {
		name = parts[1]
	}
	name = strings.ReplaceAll(name, "_", " ")
	if name == "" {
		return key
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

var (
	actStd     = []ISPAction{ActionCliente, ActionPlanos, ActionEmpresa, ActionLiberacao}                // most ISPs (no chamado)
	actFull    = []ISPAction{ActionCliente, ActionPlanos, ActionEmpresa, ActionLiberacao, ActionChamado} // + chamado
	actWHMCS   = []ISPAction{ActionCliente, ActionPlanos, ActionEmpresa, ActionChamado}                  // no liberacao
	byDoc      = []string{"cpfcnpj"}
	byDocPhone = []string{"cpfcnpj", "phone"}
	byEmail    = []string{"email"}
)

// ISPCatalog is the versioned source of truth: the 19 ISPs the smsnet-integrations
// API supports, each with its required credential fields and supported actions.
var ISPCatalog = []ISPDescriptor{
	{Slug: "altarede", Label: "Altarede", Credentials: creds("altarede_host", "altarede_token", "altarede_appkey"), Actions: actStd, SearchBy: byDoc},
	{Slug: "beesweb", Label: "BeesWeb", Credentials: creds("beesweb_host", "beesweb_email", "beesweb_password"), Actions: actStd, SearchBy: byDoc},
	{Slug: "hubsoft", Label: "Hubsoft", Credentials: creds("hubsoft_host", "hubsoft_client_id", "hubsoft_client_secret", "hubsoft_username", "hubsoft_password"), Actions: actStd, SearchBy: byDocPhone},
	{Slug: "ispcloud", Label: "ISP Cloud", Credentials: creds("ispcloud_host", "ispcloud_token"), Actions: actStd, SearchBy: byDoc},
	{Slug: "ispcontrollr", Label: "ISP Controll-R", Credentials: creds("ispcontrollr_host", "ispcontrollr_usuario", "ispcontrollr_senha"), Actions: actStd, SearchBy: byDoc}, //nolint:misspell // "Controll-R" is the vendor's branded product name.
	{Slug: "ispfy", Label: "ISPFY", Credentials: creds("ispfy_host", "ispfy_token"), Actions: actStd, SearchBy: byDocPhone},
	{Slug: "ixcsoft", Label: "IXCSoft", Credentials: creds("ixcsoft_host", "ixcsoft_token"), Actions: actFull, SearchBy: byDocPhone},
	{Slug: "mikweb", Label: "MikWeb", Credentials: creds("mikweb_host", "mikweb_token"), Actions: actStd, SearchBy: byDoc},
	{Slug: "mkauth", Label: "MK-Auth", Credentials: creds("mkauth_host", "mkauth_token"), Actions: actFull, SearchBy: byDocPhone},
	{Slug: "mksolutions", Label: "MK Solutions", Credentials: creds("mksolutions_host", "mksolutions_token", "mksolutions_password"), Actions: actStd, SearchBy: byDoc},
	{Slug: "netcontrol", Label: "NetControl", Credentials: creds("netcontrol_host", "netcontrol_client_id", "netcontrol_client_secret"), Actions: actStd, SearchBy: byDoc},
	{Slug: "radiusnet", Label: "RadiusNet", Credentials: creds("radiusnet_host", "radiusnet_rtoken"), Actions: actStd, SearchBy: byDoc},
	{Slug: "rbfull", Label: "RBFull", Credentials: creds("rbfull_host", "rbfull_token"), Actions: actStd, SearchBy: byDoc},
	{Slug: "rbxsoft", Label: "RBXSoft", Credentials: creds("rbxsoft_host", "rbxsoft_token"), Actions: actStd, SearchBy: byDoc},
	{Slug: "receitanet", Label: "ReceitaNet", Credentials: creds("receitanet_host", "receitanet_token"), Actions: actFull, SearchBy: byDocPhone},
	{Slug: "sgmcloud", Label: "SGM Cloud", Credentials: creds("sgmcloud_host", "sgmcloud_token"), Actions: actStd, SearchBy: byDoc},
	{Slug: "sgpnet", Label: "SGPNet", Credentials: creds("sgpnet_host", "sgpnet_token"), Actions: actFull, SearchBy: byDocPhone},
	{Slug: "topsapp", Label: "TopsApp", Credentials: creds("topsapp_host", "topsapp_identificador", "topsapp_usuario", "topsapp_senha"), Actions: actStd, SearchBy: byDoc},
	{Slug: "whmcs", Label: "WHMCS", Credentials: creds("whmcs_host", "whmcs_identifier", "whmcs_secret"), Actions: actWHMCS, SearchBy: byEmail},
}

// KnownISPTypes is the closed set of accepted isp_type slugs (the catalog slugs
// plus the legacy aliases), populated in init from ISPCatalog.
var KnownISPTypes []string

var knownISPSet map[string]struct{}

func init() {
	knownISPSet = make(map[string]struct{}, len(ISPCatalog)+2)
	for _, d := range ISPCatalog {
		KnownISPTypes = append(KnownISPTypes, d.Slug)
		knownISPSet[d.Slug] = struct{}{}
	}
	for _, legacy := range []string{ISPVoalle, ISPSGP} {
		KnownISPTypes = append(KnownISPTypes, legacy)
		knownISPSet[legacy] = struct{}{}
	}
}

// IsKnownISPType reports whether t is an accepted isp_type slug.
func IsKnownISPType(t string) bool {
	_, ok := knownISPSet[t]
	return ok
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

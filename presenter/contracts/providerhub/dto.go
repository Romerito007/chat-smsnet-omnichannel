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

// GatewayStatusResponse is the GET /v1/providerhub/config response. The SMSNET
// gateway is infra now (env ISP_GATEWAY_API_HOST/KEY), so this reports whether the
// gateway is configured plus a summary of the tenant's ISP profiles. The host/key
// are never returned.
type GatewayStatusResponse struct {
	Source           string `json:"source"`     // "env" | "none"
	Configured       bool   `json:"configured"` // gateway env host present
	HasProfiles      bool   `json:"has_profiles"`
	DefaultProfileID string `json:"default_profile_id,omitempty"`
	ProfilesCount    int    `json:"profiles_count"`
}

// NewGatewayStatusResponse maps the service status to the DTO.
func NewGatewayStatusResponse(st phcontracts.GatewayStatus) GatewayStatusResponse {
	return GatewayStatusResponse{
		Source:           st.Source,
		Configured:       st.Configured,
		HasProfiles:      st.HasProfiles,
		DefaultProfileID: st.DefaultProfileID,
		ProfilesCount:    st.ProfilesCount,
	}
}

// CreateProfileRequest is the body of POST /v1/providerhub/profiles.
type CreateProfileRequest struct {
	Label                       string            `json:"label"`
	ISPType                     string            `json:"isp_type"`
	Credentials                 map[string]string `json:"credentials"`
	IsDefault                   bool              `json:"is_default"`
	UsaPegarFaturaAtrasada      bool              `json:"usa_pegar_fatura_atrasada"`
	UsaExtrairLinhaDigitavelPDF bool              `json:"usa_extrair_linha_digitavel_pdf"`
	TimeoutMs                   int               `json:"timeout_ms"`
	Enabled                     *bool             `json:"enabled"`
}

// ToCommand maps to the service command.
func (r CreateProfileRequest) ToCommand() phcontracts.CreateProfile {
	return phcontracts.CreateProfile{
		Label:                       r.Label,
		ISPType:                     r.ISPType,
		Credentials:                 r.Credentials,
		IsDefault:                   r.IsDefault,
		UsaPegarFaturaAtrasada:      r.UsaPegarFaturaAtrasada,
		UsaExtrairLinhaDigitavelPDF: r.UsaExtrairLinhaDigitavelPDF,
		TimeoutMs:                   r.TimeoutMs,
		Enabled:                     r.Enabled,
	}
}

// UpdateProfileRequest is the body of PATCH /v1/providerhub/profiles/{id}. All
// fields optional; is_default is changed via the dedicated default endpoint.
type UpdateProfileRequest struct {
	Label                       *string            `json:"label"`
	ISPType                     *string            `json:"isp_type"`
	Credentials                 *map[string]string `json:"credentials"`
	UsaPegarFaturaAtrasada      *bool              `json:"usa_pegar_fatura_atrasada"`
	UsaExtrairLinhaDigitavelPDF *bool              `json:"usa_extrair_linha_digitavel_pdf"`
	TimeoutMs                   *int               `json:"timeout_ms"`
	Enabled                     *bool              `json:"enabled"`
}

// ToCommand maps to the service command.
func (r UpdateProfileRequest) ToCommand() phcontracts.UpdateProfile {
	return phcontracts.UpdateProfile{
		Label:                       r.Label,
		ISPType:                     r.ISPType,
		Credentials:                 r.Credentials,
		UsaPegarFaturaAtrasada:      r.UsaPegarFaturaAtrasada,
		UsaExtrairLinhaDigitavelPDF: r.UsaExtrairLinhaDigitavelPDF,
		TimeoutMs:                   r.TimeoutMs,
		Enabled:                     r.Enabled,
	}
}

// ProfileResponse is the public representation of an ISP profile. Credentials are
// masked: only their keys are exposed (never values). actions[] is derived from
// the catalog so the front can gate per-ISP actions.
type ProfileResponse struct {
	ID                          string    `json:"id"`
	TenantID                    string    `json:"tenant_id"`
	Label                       string    `json:"label"`
	ISPType                     string    `json:"isp_type"`
	CredentialKeys              []string  `json:"credential_keys"`
	IsDefault                   bool      `json:"is_default"`
	Actions                     []string  `json:"actions"`
	UsaPegarFaturaAtrasada      bool      `json:"usa_pegar_fatura_atrasada"`
	UsaExtrairLinhaDigitavelPDF bool      `json:"usa_extrair_linha_digitavel_pdf"`
	TimeoutMs                   int       `json:"timeout_ms"`
	Enabled                     bool      `json:"enabled"`
	CreatedAt                   time.Time `json:"created_at"`
	UpdatedAt                   time.Time `json:"updated_at"`
}

// NewProfileResponse maps a profile entity, masking credentials and attaching the
// catalog actions[] for its isp_type.
func NewProfileResponse(p *phentity.ISPProfile) ProfileResponse {
	keys := make([]string, 0, len(p.Credentials))
	for k := range p.Credentials {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	actions := make([]string, 0)
	for _, a := range p.Actions() {
		actions = append(actions, string(a))
	}
	return ProfileResponse{
		ID:                          p.ID,
		TenantID:                    p.TenantID,
		Label:                       p.Label,
		ISPType:                     p.ISPType,
		CredentialKeys:              keys,
		IsDefault:                   p.IsDefault,
		Actions:                     actions,
		UsaPegarFaturaAtrasada:      p.Options.UsaPegarFaturaAtrasada,
		UsaExtrairLinhaDigitavelPDF: p.Options.UsaExtrairLinhaDigitavelPDF,
		TimeoutMs:                   p.TimeoutMs,
		Enabled:                     p.Enabled,
		CreatedAt:                   p.CreatedAt,
		UpdatedAt:                   p.UpdatedAt,
	}
}

// NewProfileListResponse maps a slice of profiles to a { data: [...] } envelope.
func NewProfileListResponse(ps []*phentity.ISPProfile) map[string]any {
	out := make([]ProfileResponse, 0, len(ps))
	for _, p := range ps {
		out = append(out, NewProfileResponse(p))
	}
	return map[string]any{"data": out}
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

// ── ISP catalog (GET /v1/providerhub/catalog) ────────────────────────────────

// CatalogResponse is the static, versioned catalog of supported ISPs: per ISP,
// the credential fields the UI must render and the actions it supports. The
// backend is the single source of truth so the front hard-codes nothing.
type CatalogResponse struct {
	Version string            `json:"version"`
	ISPs    []ISPCatalogEntry `json:"isps"`
}

// ISPCatalogEntry is one ISP in the catalog.
type ISPCatalogEntry struct {
	Slug        string               `json:"slug"`
	Label       string               `json:"label"`
	Credentials []ISPCredentialEntry `json:"credentials"`
	Actions     []string             `json:"actions"`   // cliente|planos|empresa|liberacao|chamado
	SearchBy    []string             `json:"search_by"` // cpfcnpj|phone|email
}

// ISPCredentialEntry is one credential input for an ISP. Secret fields must be
// rendered masked; the value is never echoed back by the config endpoints.
type ISPCredentialEntry struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Secret bool   `json:"secret"`
}

// NewCatalogResponse maps the entity catalog (source of truth) to the DTO.
func NewCatalogResponse() CatalogResponse {
	out := CatalogResponse{Version: phentity.ISPCatalogVersion, ISPs: make([]ISPCatalogEntry, 0, len(phentity.ISPCatalog))}
	for _, d := range phentity.ISPCatalog {
		entry := ISPCatalogEntry{Slug: d.Slug, Label: d.Label}
		for _, c := range d.Credentials {
			entry.Credentials = append(entry.Credentials, ISPCredentialEntry{Key: c.Key, Label: c.Label, Secret: c.Secret})
		}
		for _, a := range d.Actions {
			entry.Actions = append(entry.Actions, string(a))
		}
		entry.SearchBy = append(entry.SearchBy, d.SearchBy...)
		out.ISPs = append(out.ISPs, entry)
	}
	return out
}

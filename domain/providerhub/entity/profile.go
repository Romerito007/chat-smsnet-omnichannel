package entity

import "time"

// Transport is a way the copilot/manual path can reach the SMSNET integration for a
// profile. The shared SMSNET surfaces are infra (env): HTTP is the ProviderHub
// gateway (ISP_GATEWAY_API_HOST); MCP is the CONSULTAS/OPERACOES servers. A profile
// only ENABLES transports — it never carries the addresses.
const (
	TransportHTTP = "http"
	TransportMCP  = "mcp"
)

// IsSupportedTransport reports whether t is a known transport slug.
func IsSupportedTransport(t string) bool { return t == TransportHTTP || t == TransportMCP }

// NormalizeTransports trims, lowercases (canonical slugs are already lowercase),
// de-duplicates and orders the transports as http,mcp. ok is false when any value
// is unknown OR the result is empty — the caller surfaces a 422 (a profile must
// enable at least one valid transport; there is NO silent default).
func NormalizeTransports(in []string) (out []string, ok bool) {
	var http, mcp bool
	for _, t := range in {
		switch t {
		case TransportHTTP:
			http = true
		case TransportMCP:
			mcp = true
		default:
			return nil, false // unknown transport
		}
	}
	if http {
		out = append(out, TransportHTTP)
	}
	if mcp {
		out = append(out, TransportMCP)
	}
	return out, len(out) > 0
}

// ISPProfile is one addressable ISP configuration a tenant can hold (many per
// tenant). It carries only the ISP identity and its credentials — NOT the SMSNET
// gateway host/key, which stay infra (env ISP_GATEWAY_API_HOST/KEY). The resolver
// combines the env gateway with a profile to make an effective call config.
//
// Credentials are encrypted at rest and never returned; only their keys are
// exposed (via the presenter). At most one profile per tenant has IsDefault=true,
// enforced by a partial-unique index.
type ISPProfile struct {
	ID          string
	TenantID    string
	Label       string            // human label to distinguish profiles ("IXC matriz", "SGP filial")
	ISPType     string            // isp_type slug, validated against ISPCatalog
	Credentials map[string]string // isp credential map (encrypted at rest); keys match catalog[isp_type]
	IsDefault   bool
	Options     Options // only the two toggles are used (DadosPlanos/DadosEmpresa are not part of a profile)
	TimeoutMs   int
	Enabled     bool
	// Transports are the SMSNET surfaces this profile enables (subset of
	// {http, mcp}). It is REQUIRED (validated non-empty); the addresses come from
	// env. The manual search (/external/*) needs "http"; an assistant pinning this
	// profile may only select a transport listed here.
	Transports []string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Actions returns the actions this profile's ISP supports, per the catalog.
func (p *ISPProfile) Actions() []ISPAction { return ActionsFor(p.ISPType) }

// SupportsTransport reports whether the profile enables the given transport.
func (p *ISPProfile) SupportsTransport(t string) bool {
	for _, x := range p.Transports {
		if x == t {
			return true
		}
	}
	return false
}

// SupportsHTTP reports whether the profile enables the HTTP gateway transport
// (required for the manual search path).
func (p *ISPProfile) SupportsHTTP() bool { return p.SupportsTransport(TransportHTTP) }

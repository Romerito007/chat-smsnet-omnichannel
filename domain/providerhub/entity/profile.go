package entity

import "time"

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
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Actions returns the actions this profile's ISP supports, per the catalog.
func (p *ISPProfile) Actions() []ISPAction { return ActionsFor(p.ISPType) }

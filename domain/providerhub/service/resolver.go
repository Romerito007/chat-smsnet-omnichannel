package service

import (
	"context"
	"errors"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// ResolveStatus is the outcome of resolving which ISP profile a call should use.
type ResolveStatus int

const (
	// ResolveOK: a single profile was selected (explicit id or the default).
	ResolveOK ResolveStatus = iota
	// ResolveAmbiguous: no default and 2+ eligible profiles — the caller must pick.
	ResolveAmbiguous
	// ResolveNone: the tenant has no eligible profile — external actions are off.
	ResolveNone
)

// ResolveResult is the outcome of ISPResolver.Resolve.
type ResolveResult struct {
	Status   ResolveStatus
	Profile  *entity.ISPProfile   // set when ResolveOK
	Eligible []*entity.ISPProfile // set when ResolveAmbiguous
}

// ISPResolver picks the ISP profile for an external call: an explicit profile id
// wins; otherwise the tenant default; otherwise it reports ambiguity (eligible
// list) or none. It never lets ISP credentials cross an AI decision layer — it
// only selects a profile; the gateway call config is built at the edge.
type ISPResolver struct {
	profiles repository.ProfileRepository
}

// NewISPResolver builds the resolver.
func NewISPResolver(profiles repository.ProfileRepository) *ISPResolver {
	return &ISPResolver{profiles: profiles}
}

// Resolve selects the profile for the MANUAL (HTTP gateway) path, so only profiles
// that ENABLE the http transport are eligible — a mcp-only profile cannot serve the
// manual search. explicitID (when non-empty) must be an enabled, http-enabled
// profile of the tenant, else a NotFound / Conflict error. With no explicit id: an
// http-enabled default wins; otherwise a single http-enabled profile resolves; 2+
// is ResolveAmbiguous; zero is ResolveNone.
func (r *ISPResolver) Resolve(ctx context.Context, explicitID string) (ResolveResult, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return ResolveResult{}, err
	}
	explicitID = strings.TrimSpace(explicitID)
	if explicitID != "" {
		p, err := r.profiles.FindByID(ctx, explicitID)
		if err != nil {
			return ResolveResult{}, err
		}
		if !p.Enabled {
			return ResolveResult{}, apperror.NotFound("isp profile not found")
		}
		if !p.SupportsHTTP() {
			return ResolveResult{}, apperror.Conflict("este perfil de ISP não habilita o transporte HTTP, exigido pela busca manual")
		}
		return ResolveResult{Status: ResolveOK, Profile: p}, nil
	}

	all, err := r.profiles.List(ctx)
	if err != nil {
		return ResolveResult{}, err
	}
	eligible := make([]*entity.ISPProfile, 0, len(all))
	for _, p := range all {
		if !p.Enabled || !p.SupportsHTTP() {
			continue // manual search needs the http transport
		}
		eligible = append(eligible, p)
		if p.IsDefault {
			return ResolveResult{Status: ResolveOK, Profile: p}, nil
		}
	}
	switch len(eligible) {
	case 0:
		return ResolveResult{Status: ResolveNone}, nil
	case 1:
		return ResolveResult{Status: ResolveOK, Profile: eligible[0]}, nil
	default:
		return ResolveResult{Status: ResolveAmbiguous, Eligible: eligible}, nil
	}
}

// ISPSelectionRequiredError is returned by the query service when the ISP profile
// is ambiguous (no default, 2+ eligible). The controller maps it to a 200
// needs_isp_selection response carrying the eligible profiles.
type ISPSelectionRequiredError struct {
	Eligible []*entity.ISPProfile
}

func (e *ISPSelectionRequiredError) Error() string { return "isp selection required" }

// AsISPSelectionRequired reports whether err is an ISPSelectionRequiredError.
func AsISPSelectionRequired(err error) (*ISPSelectionRequiredError, bool) {
	var sel *ISPSelectionRequiredError
	if errors.As(err, &sel) {
		return sel, true
	}
	return nil, false
}

// buildCallConfig assembles the effective gateway call config: the infra gateway
// (env host/key) + the profile's ISP identity/credentials/options. botId defaults
// to the tenant id.
func buildCallConfig(tenantID, envHost, envKey string, p *entity.ISPProfile) *entity.ProviderIntegrationConfig {
	return &entity.ProviderIntegrationConfig{
		TenantID:       tenantID,
		SMSNetBaseURL:  envHost,
		SMSNetAPIKey:   envKey,
		ISPType:        p.ISPType,
		ISPCredentials: p.Credentials,
		Options: entity.Options{
			UsaPegarFaturaAtrasada:      p.Options.UsaPegarFaturaAtrasada,
			UsaExtrairLinhaDigitavelPDF: p.Options.UsaExtrairLinhaDigitavelPDF,
		},
		Enabled:   true,
		TimeoutMs: p.TimeoutMs,
	}
}

// Package service holds the providerhub business logic: ISP-profile management,
// the on-demand smsnet-integrations gateway and (legacy) single-config resolution.
// It never persists external payloads.
package service

import (
	"context"
	"sort"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// ProfileService manages the per-tenant ISP profiles and reports gateway status.
type ProfileService struct {
	repo    repository.ProfileRepository
	gateway contracts.Gateway
	clock   shared.Clock
	envHost string
	envKey  string
}

// NewProfileService builds the service.
func NewProfileService(repo repository.ProfileRepository, gateway contracts.Gateway, clock shared.Clock) *ProfileService {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &ProfileService{repo: repo, gateway: gateway, clock: clock}
}

// SetEnvDefault wires the infra SMSNET gateway host/key (ISP_GATEWAY_API_HOST/KEY).
func (s *ProfileService) SetEnvDefault(host, key string) {
	s.envHost, s.envKey = host, key
}

// List returns the tenant's profiles.
func (s *ProfileService) List(ctx context.Context) ([]*entity.ISPProfile, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.List(ctx)
}

// Get returns one profile by id (tenant-scoped).
func (s *ProfileService) Get(ctx context.Context, id string) (*entity.ISPProfile, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}

// Create registers a new ISP profile. The first profile of a tenant is forced to
// be the default. Setting IsDefault unsets any previous default first.
func (s *ProfileService) Create(ctx context.Context, cmd contracts.CreateProfile) (*entity.ISPProfile, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	label := strings.TrimSpace(cmd.Label)
	ispType := strings.TrimSpace(cmd.ISPType)
	if err := validateProfile(label, ispType, cmd.Credentials); err != nil {
		return nil, err
	}

	existing, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	isDefault := cmd.IsDefault || len(existing) == 0 // first profile is always the default
	if isDefault {
		if err := s.repo.ClearDefault(ctx); err != nil {
			return nil, err
		}
	}

	timeout := cmd.TimeoutMs
	if timeout <= 0 {
		timeout = defaultTimeoutMs
	}
	enabled := true
	if cmd.Enabled != nil {
		enabled = *cmd.Enabled
	}
	now := s.clock.Now()
	p := &entity.ISPProfile{
		ID:          shared.NewID(),
		TenantID:    tenantID,
		Label:       label,
		ISPType:     ispType,
		Credentials: cmd.Credentials,
		IsDefault:   isDefault,
		Options: entity.Options{
			UsaPegarFaturaAtrasada:      cmd.UsaPegarFaturaAtrasada,
			UsaExtrairLinhaDigitavelPDF: cmd.UsaExtrairLinhaDigitavelPDF,
		},
		TimeoutMs: timeout,
		Enabled:   enabled,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// Update applies the non-nil fields of cmd. When isp_type and/or credentials
// change, the resulting (type, credentials) pair is re-validated against the
// catalog.
func (s *ProfileService) Update(ctx context.Context, id string, cmd contracts.UpdateProfile) (*entity.ISPProfile, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	p, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if cmd.Label != nil {
		p.Label = strings.TrimSpace(*cmd.Label)
	}
	if cmd.ISPType != nil {
		p.ISPType = strings.TrimSpace(*cmd.ISPType)
	}
	if cmd.Credentials != nil {
		p.Credentials = *cmd.Credentials
	}
	if err := validateProfile(p.Label, p.ISPType, p.Credentials); err != nil {
		return nil, err
	}
	if cmd.UsaPegarFaturaAtrasada != nil {
		p.Options.UsaPegarFaturaAtrasada = *cmd.UsaPegarFaturaAtrasada
	}
	if cmd.UsaExtrairLinhaDigitavelPDF != nil {
		p.Options.UsaExtrairLinhaDigitavelPDF = *cmd.UsaExtrairLinhaDigitavelPDF
	}
	if cmd.TimeoutMs != nil && *cmd.TimeoutMs > 0 {
		p.TimeoutMs = *cmd.TimeoutMs
	}
	if cmd.Enabled != nil {
		p.Enabled = *cmd.Enabled
	}
	p.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// Delete removes a profile.
func (s *ProfileService) Delete(ctx context.Context, id string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// SetDefault makes the given profile the tenant's default (unsetting any previous
// default first, so the partial-unique index never sees two defaults).
func (s *ProfileService) SetDefault(ctx context.Context, id string) (*entity.ISPProfile, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	p, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.IsDefault {
		return p, nil
	}
	if err := s.repo.ClearDefault(ctx); err != nil {
		return nil, err
	}
	p.IsDefault = true
	p.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// Test pings the SMSNET gateway with the env host/key + this profile's ISP config.
func (s *ProfileService) Test(ctx context.Context, id string) (contracts.TestResult, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return contracts.TestResult{}, err
	}
	p, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return contracts.TestResult{}, err
	}
	if s.envHost == "" {
		return contracts.TestResult{}, apperror.Integration("smsnet gateway is not configured")
	}
	cfg := s.callConfig(ctx, p)
	start := s.clock.Now()
	perr := s.gateway.Ping(ctx, cfg)
	latency := s.clock.Now().Sub(start).Milliseconds()
	if perr != nil {
		return contracts.TestResult{OK: false, LatencyMs: latency, Error: "could not reach the provider API"}, nil
	}
	return contracts.TestResult{OK: true, LatencyMs: latency}, nil
}

// GatewayStatus reports gateway availability (infra/env) + a profile summary.
func (s *ProfileService) GatewayStatus(ctx context.Context) (contracts.GatewayStatus, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return contracts.GatewayStatus{}, err
	}
	st := contracts.GatewayStatus{Source: "none"}
	if s.envHost != "" {
		st.Source = "env"
		st.Configured = true
	}
	profiles, err := s.repo.List(ctx)
	if err != nil {
		return contracts.GatewayStatus{}, err
	}
	st.ProfilesCount = len(profiles)
	st.HasProfiles = len(profiles) > 0
	for _, p := range profiles {
		if p.IsDefault {
			st.DefaultProfileID = p.ID
			break
		}
	}
	return st, nil
}

// callConfig builds the effective gateway call config: infra gateway (env
// host/key) + the profile's ISP identity/credentials/options. botId defaults to
// the tenant id.
func (s *ProfileService) callConfig(ctx context.Context, p *entity.ISPProfile) *entity.ProviderIntegrationConfig {
	tenantID, _ := shared.TenantFrom(ctx)
	return &entity.ProviderIntegrationConfig{
		TenantID:       tenantID,
		SMSNetBaseURL:  s.envHost,
		SMSNetAPIKey:   s.envKey,
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

// validateProfile enforces a non-empty label, an isp_type from the catalog (the
// 19 slugs; legacy aliases are not allowed for new profiles) and that the
// credential keys match the catalog 1:1 (no missing, no unknown).
func validateProfile(label, ispType string, creds map[string]string) error {
	v := map[string]any{}
	if label == "" {
		v["label"] = "is required"
	}
	if ispType == "" {
		v["isp_type"] = "is required"
	} else if _, ok := entity.DescriptorFor(ispType); !ok {
		v["isp_type"] = "unknown isp_type; expected one of " + strings.Join(catalogSlugs(), ", ")
	} else if msg := credentialKeyMismatch(ispType, creds); msg != "" {
		v["isp_credentials"] = msg
	}
	if len(v) > 0 {
		return apperror.Validation("invalid ISP profile").WithDetails(v)
	}
	return nil
}

// credentialKeyMismatch returns "" when the provided credential keys match the
// catalog keys for ispType exactly, else a message naming the missing/unknown keys.
func credentialKeyMismatch(ispType string, creds map[string]string) string {
	want := map[string]struct{}{}
	for _, k := range entity.CredentialKeysFor(ispType) {
		want[k] = struct{}{}
	}
	var missing, unknown []string
	for k := range want {
		if _, ok := creds[k]; !ok {
			missing = append(missing, k)
		}
	}
	for k := range creds {
		if _, ok := want[k]; !ok {
			unknown = append(unknown, k)
		}
	}
	sort.Strings(missing)
	sort.Strings(unknown)
	var parts []string
	if len(missing) > 0 {
		parts = append(parts, "missing "+strings.Join(missing, ", "))
	}
	if len(unknown) > 0 {
		parts = append(parts, "unknown "+strings.Join(unknown, ", "))
	}
	return strings.Join(parts, "; ")
}

// catalogSlugs lists the catalog slugs (sorted) for error messages.
func catalogSlugs() []string {
	out := make([]string, 0, len(entity.ISPCatalog))
	for _, d := range entity.ISPCatalog {
		out = append(out, d.Slug)
	}
	return out
}

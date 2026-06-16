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

// ProfileUsageChecker reports whether an ISP profile is referenced by a consumer
// (e.g. a CopilotAssistant), so a delete can be blocked with a clear message.
// Implemented by the copilot side; wired optionally.
type ProfileUsageChecker interface {
	IsISPProfileInUse(ctx context.Context, ispProfileID string) (inUse bool, usedBy string, err error)
}

// ProfileService manages the per-tenant ISP profiles and reports gateway status.
type ProfileService struct {
	repo    repository.ProfileRepository
	gateway contracts.Gateway
	clock   shared.Clock
	envHost string
	envKey  string
	usage   ProfileUsageChecker
}

// SetUsageChecker wires the referential-integrity checker used to block deleting a
// profile that is in use. Optional.
func (s *ProfileService) SetUsageChecker(c ProfileUsageChecker) { s.usage = c }

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
	transports, tok := entity.NormalizeTransports(cmd.Transports)
	if err := validateProfile(label, ispType, cmd.Credentials, transports, tok); err != nil {
		return nil, err
	}
	// EnabledActions default = all of the ISP's catalog actions (the common case is
	// "offers everything"; the tenant unchecks what it doesn't want). A provided set
	// is validated as a subset of the catalog.
	enabledActions, err := resolveEnabledActions(ispType, cmd.EnabledActions, cmd.EnabledActions != nil)
	if err != nil {
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
		ID:             shared.NewID(),
		TenantID:       tenantID,
		Label:          label,
		ISPType:        ispType,
		Credentials:    cmd.Credentials,
		Transports:     transports,
		EnabledActions: enabledActions,
		IsDefault:      isDefault,
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
	transports, tok := p.Transports, true
	if cmd.Transports != nil {
		transports, tok = entity.NormalizeTransports(*cmd.Transports)
	}
	if err := validateProfile(p.Label, p.ISPType, p.Credentials, transports, tok); err != nil {
		return nil, err
	}
	p.Transports = transports
	// EnabledActions: an explicit set replaces (validated as a subset of the ISP's
	// catalog); otherwise, when the isp_type changed, re-filter the existing set to
	// the new catalog so it never references actions the new ISP doesn't offer.
	switch {
	case cmd.EnabledActions != nil:
		actions, aerr := resolveEnabledActions(p.ISPType, *cmd.EnabledActions, true)
		if aerr != nil {
			return nil, aerr
		}
		p.EnabledActions = actions
	case cmd.ISPType != nil:
		actions, _ := entity.NormalizeActions(p.EnabledActions, entity.ActionsFor(p.ISPType))
		p.EnabledActions = actions
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

// Delete removes a profile. Predictable default promotion: if the delete left the
// tenant without a default and EXACTLY ONE profile remains, that one is promoted.
// With 2+ remaining we do NOT guess — the tenant is left without a default
// (GET /config → default_profile_id null) and the UI prompts to pick one.
func (s *ProfileService) Delete(ctx context.Context, id string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	// Referential integrity: refuse to delete a profile a CopilotAssistant pins,
	// with a clear message — never silently break the assistant.
	if s.usage != nil {
		inUse, usedBy, err := s.usage.IsISPProfileInUse(ctx, id)
		if err != nil {
			return err
		}
		if inUse {
			return apperror.Conflict("ISP em uso pelo assistente " + usedBy)
		}
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	remaining, err := s.repo.List(ctx)
	if err != nil {
		return err
	}
	if len(remaining) != 1 {
		return nil // 0 left, or 2+ and we won't guess which becomes default
	}
	only := remaining[0]
	if only.IsDefault {
		return nil // a default still exists (deleted profile was not the default)
	}
	only.IsDefault = true
	only.UpdatedAt = s.clock.Now()
	return s.repo.Update(ctx, only)
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

// callConfig builds the effective gateway call config for a test (env gateway +
// profile).
func (s *ProfileService) callConfig(ctx context.Context, p *entity.ISPProfile) *entity.ProviderIntegrationConfig {
	tenantID, _ := shared.TenantFrom(ctx)
	return buildCallConfig(tenantID, s.envHost, s.envKey, p)
}

// validateProfile enforces a non-empty label, an isp_type from the catalog (the
// 19 slugs; legacy aliases are not allowed for new profiles) and that the
// credential keys match the catalog 1:1 (no missing, no unknown).
func validateProfile(label, ispType string, creds map[string]string, transports []string, transportsOK bool) error {
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
	// transports is REQUIRED: at least one valid transport (http and/or mcp). An
	// unknown value or an empty set is rejected — no silent default.
	if !transportsOK || len(transports) == 0 {
		v["transports"] = "must enable at least one valid transport (http and/or mcp)"
	}
	if len(v) > 0 {
		return apperror.Validation("invalid ISP profile").WithDetails(v)
	}
	return nil
}

// resolveEnabledActions returns the actions a profile offers. When the client did
// not provide a set (provided=false), it defaults to ALL of the ISP's catalog
// actions. When provided, it validates the set as a subset of the catalog (unknown
// action → 422). The ISP type is assumed already validated by validateProfile.
func resolveEnabledActions(ispType string, in []string, provided bool) ([]string, error) {
	allowed := entity.ActionsFor(ispType)
	if !provided {
		return entity.ActionSlugs(allowed), nil // default = everything the ISP offers
	}
	out, ok := entity.NormalizeActions(in, allowed)
	if !ok {
		return nil, apperror.Validation("invalid enabled_actions").WithDetails(map[string]any{
			"enabled_actions": "must be a subset of " + strings.Join(entity.ActionSlugs(allowed), ", "),
		})
	}
	return out, nil
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

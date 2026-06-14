package service

import (
	"context"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	chrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/repository"
	mcprepo "github.com/romerito007/chat-smsnet-omnichannel/domain/mcp/repository"
	phrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// CreateAssistant is the input to AssistantService.Create.
type CreateAssistant struct {
	Name         string
	ChannelIDs   []string
	ISPProfileID string
	MCPServerID  string
	Enabled      *bool // nil → true
}

// UpdateAssistant carries optional fields; nil pointers mean "leave unchanged".
type UpdateAssistant struct {
	Name         *string
	ChannelIDs   *[]string
	ISPProfileID *string
	MCPServerID  *string
	Enabled      *bool
}

// AssistantService manages copilot assistants (many per tenant). It validates that
// the served channels exist and that the assistant's external tool source — the
// pinned ISP profile OR the custom MCP server (mutually exclusive) — exists. It
// also answers whether a profile / MCP server is in use (so providerhub / the MCP
// admin can block deleting a referenced one).
type AssistantService struct {
	repo     repository.AssistantRepository
	profiles phrepo.ProfileRepository
	channels chrepo.ConnectionRepository
	servers  mcprepo.ServerRepository
	clock    shared.Clock
}

// NewAssistantService builds the service.
func NewAssistantService(repo repository.AssistantRepository, profiles phrepo.ProfileRepository, channels chrepo.ConnectionRepository, servers mcprepo.ServerRepository, clock shared.Clock) *AssistantService {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &AssistantService{repo: repo, profiles: profiles, channels: channels, servers: servers, clock: clock}
}

// List returns the tenant's assistants.
func (s *AssistantService) List(ctx context.Context) ([]*entity.Assistant, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.List(ctx)
}

// Get returns one assistant by id.
func (s *AssistantService) Get(ctx context.Context, id string) (*entity.Assistant, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}

// Create registers a new assistant.
func (s *AssistantService) Create(ctx context.Context, cmd CreateAssistant) (*entity.Assistant, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return nil, apperror.Validation("name is required")
	}
	ispProfileID := strings.TrimSpace(cmd.ISPProfileID)
	mcpServerID := strings.TrimSpace(cmd.MCPServerID)
	if err := s.validateToolSource(ctx, ispProfileID, mcpServerID); err != nil {
		return nil, err
	}
	if err := s.validateChannels(ctx, cmd.ChannelIDs); err != nil {
		return nil, err
	}
	enabled := true
	if cmd.Enabled != nil {
		enabled = *cmd.Enabled
	}
	now := s.clock.Now()
	a := &entity.Assistant{
		ID:           shared.NewID(),
		TenantID:     tenantID,
		Name:         name,
		ChannelIDs:   cmd.ChannelIDs,
		ISPProfileID: ispProfileID,
		MCPServerID:  mcpServerID,
		Enabled:      enabled,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.repo.Create(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// Update applies the non-nil fields of cmd.
func (s *AssistantService) Update(ctx context.Context, id string, cmd UpdateAssistant) (*entity.Assistant, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	a, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if cmd.Name != nil {
		a.Name = strings.TrimSpace(*cmd.Name)
		if a.Name == "" {
			return nil, apperror.Validation("name is required")
		}
	}
	if cmd.ChannelIDs != nil {
		if err := s.validateChannels(ctx, *cmd.ChannelIDs); err != nil {
			return nil, err
		}
		a.ChannelIDs = *cmd.ChannelIDs
	}
	// The tool source (ISP profile XOR MCP server) is validated on the RESULTING
	// state, so a PATCH that sets one must clear/keep the other consistently.
	if cmd.ISPProfileID != nil {
		a.ISPProfileID = strings.TrimSpace(*cmd.ISPProfileID)
	}
	if cmd.MCPServerID != nil {
		a.MCPServerID = strings.TrimSpace(*cmd.MCPServerID)
	}
	if cmd.ISPProfileID != nil || cmd.MCPServerID != nil {
		if err := s.validateToolSource(ctx, a.ISPProfileID, a.MCPServerID); err != nil {
			return nil, err
		}
	}
	if cmd.Enabled != nil {
		a.Enabled = *cmd.Enabled
	}
	a.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// Delete removes an assistant.
func (s *AssistantService) Delete(ctx context.Context, id string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// IsISPProfileInUse reports whether any assistant references the ISP profile, and
// the name of one such assistant (for a clear "in use" message). Implements the
// providerhub ProfileUsageChecker port.
func (s *AssistantService) IsISPProfileInUse(ctx context.Context, ispProfileID string) (bool, string, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return false, "", err
	}
	all, err := s.repo.List(ctx)
	if err != nil {
		return false, "", err
	}
	for _, a := range all {
		if a.ISPProfileID == ispProfileID {
			return true, a.Name, nil
		}
	}
	return false, "", nil
}

// IsMCPServerInUse reports whether any assistant references the MCP server, and the
// name of one such assistant (for a clear "in use" message). Implements the MCP
// ServerUsageChecker port so a server delete can be blocked with 409.
func (s *AssistantService) IsMCPServerInUse(ctx context.Context, mcpServerID string) (bool, string, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return false, "", err
	}
	all, err := s.repo.List(ctx)
	if err != nil {
		return false, "", err
	}
	for _, a := range all {
		if a.MCPServerID == mcpServerID {
			return true, a.Name, nil
		}
	}
	return false, "", nil
}

// validateToolSource enforces that the external tool source is an ISP profile XOR a
// custom MCP server (never both), and that whichever is set exists for the tenant.
// Both empty is allowed (no external tools).
func (s *AssistantService) validateToolSource(ctx context.Context, ispProfileID, mcpServerID string) error {
	if ispProfileID != "" && mcpServerID != "" {
		return apperror.Validation("choose an ISP profile OR an MCP server, not both").
			WithDetails(map[string]any{"isp_profile_id": "mutually exclusive with mcp_server_id"})
	}
	if err := s.validateProfile(ctx, ispProfileID); err != nil {
		return err
	}
	return s.validateMCPServer(ctx, mcpServerID)
}

// validateProfile ensures a pinned ISP profile exists for the tenant (empty = no
// ISP, allowed).
func (s *AssistantService) validateProfile(ctx context.Context, ispProfileID string) error {
	if ispProfileID == "" {
		return nil
	}
	if _, err := s.profiles.FindByID(ctx, ispProfileID); err != nil {
		if apperror.From(err).Code == apperror.CodeNotFound {
			return apperror.Validation("unknown isp_profile_id")
		}
		return err
	}
	return nil
}

// validateMCPServer ensures the referenced MCP server exists for the tenant (empty
// = no MCP server, allowed).
func (s *AssistantService) validateMCPServer(ctx context.Context, mcpServerID string) error {
	if mcpServerID == "" {
		return nil
	}
	if s.servers == nil {
		return nil
	}
	if _, err := s.servers.FindByID(ctx, mcpServerID); err != nil {
		if apperror.From(err).Code == apperror.CodeNotFound {
			return apperror.Validation("unknown mcp_server_id")
		}
		return err
	}
	return nil
}

// validateChannels ensures every served channel id is an existing connection of
// the tenant.
func (s *AssistantService) validateChannels(ctx context.Context, channelIDs []string) error {
	for _, id := range channelIDs {
		if strings.TrimSpace(id) == "" {
			return apperror.Validation("channel_ids must not contain empty ids")
		}
		if _, err := s.channels.FindByID(ctx, id); err != nil {
			if apperror.From(err).Code == apperror.CodeNotFound {
				return apperror.Validation("unknown channel id: " + id)
			}
			return err
		}
	}
	return nil
}

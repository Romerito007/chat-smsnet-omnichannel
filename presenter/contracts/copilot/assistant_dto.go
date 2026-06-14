package copilot

import (
	"time"

	centity "github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
	cservice "github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/service"
)

// AssistantResponse is the public representation of a copilot assistant.
type AssistantResponse struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	Name         string    `json:"name"`
	ChannelIDs   []string  `json:"channel_ids"`
	ISPProfileID string    `json:"isp_profile_id,omitempty"`
	MCPServerID  string    `json:"mcp_server_id,omitempty"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// NewAssistantResponse maps an assistant entity to the DTO.
func NewAssistantResponse(a *centity.Assistant) AssistantResponse {
	channels := a.ChannelIDs
	if channels == nil {
		channels = []string{}
	}
	return AssistantResponse{
		ID:           a.ID,
		TenantID:     a.TenantID,
		Name:         a.Name,
		ChannelIDs:   channels,
		ISPProfileID: a.ISPProfileID,
		MCPServerID:  a.MCPServerID,
		Enabled:      a.Enabled,
		CreatedAt:    a.CreatedAt,
		UpdatedAt:    a.UpdatedAt,
	}
}

// NewAssistantListResponse maps a slice of assistants to a { data: [...] } envelope.
func NewAssistantListResponse(as []*centity.Assistant) map[string]any {
	out := make([]AssistantResponse, 0, len(as))
	for _, a := range as {
		out = append(out, NewAssistantResponse(a))
	}
	return map[string]any{"data": out}
}

// CreateAssistantRequest is the body of POST /v1/copilot/assistants.
type CreateAssistantRequest struct {
	Name         string   `json:"name"`
	ChannelIDs   []string `json:"channel_ids"`
	ISPProfileID string   `json:"isp_profile_id"`
	MCPServerID  string   `json:"mcp_server_id"`
	Enabled      *bool    `json:"enabled"`
}

// ToCommand maps to the service command.
func (r CreateAssistantRequest) ToCommand() cservice.CreateAssistant {
	return cservice.CreateAssistant{
		Name:         r.Name,
		ChannelIDs:   r.ChannelIDs,
		ISPProfileID: r.ISPProfileID,
		MCPServerID:  r.MCPServerID,
		Enabled:      r.Enabled,
	}
}

// UpdateAssistantRequest is the body of PATCH /v1/copilot/assistants/{id}.
type UpdateAssistantRequest struct {
	Name         *string   `json:"name"`
	ChannelIDs   *[]string `json:"channel_ids"`
	ISPProfileID *string   `json:"isp_profile_id"`
	MCPServerID  *string   `json:"mcp_server_id"`
	Enabled      *bool     `json:"enabled"`
}

// ToCommand maps to the service command.
func (r UpdateAssistantRequest) ToCommand() cservice.UpdateAssistant {
	return cservice.UpdateAssistant{
		Name:         r.Name,
		ChannelIDs:   r.ChannelIDs,
		ISPProfileID: r.ISPProfileID,
		MCPServerID:  r.MCPServerID,
		Enabled:      r.Enabled,
	}
}

package entity

import "time"

// Assistant is a named copilot assistant a tenant can define (many per tenant).
// It reuses the tenant's AIConfig for provider/key/policies and adds routing: the
// specific channel connections it serves and its EXTERNAL TOOL SOURCE — either a
// pinned ISP profile (SMSNET tools, injected server-side) OR a custom MCP server,
// never both. With neither, the assistant offers no external tools.
type Assistant struct {
	ID         string
	TenantID   string
	Name       string
	ChannelIDs []string // ids of the ChannelConnections this assistant serves
	// ISPProfileID and MCPServerID are the MUTUALLY EXCLUSIVE external tool source:
	//   - ISPProfileID set → SMSNET tools (gated/injected from the ISP profile).
	//   - MCPServerID set  → the tools of that tenant-registered MCP server only.
	//   - both empty       → no external tools.
	ISPProfileID string
	MCPServerID  string
	Enabled      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ServesChannelID reports whether the assistant serves the given channel
// connection id.
func (a *Assistant) ServesChannelID(channelID string) bool {
	for _, c := range a.ChannelIDs {
		if c == channelID {
			return true
		}
	}
	return false
}

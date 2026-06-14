package entity

import "time"

// Assistant is a named copilot assistant a tenant can define (many per tenant).
// It reuses the tenant's AIConfig for provider/key/policies and adds routing: the
// specific channel connections it serves and an optional pinned ISP profile. With
// an ISP profile the backend exposes the SMSNET tools to the model and injects the
// ISP config server-side; without one, no ISP tools are offered.
type Assistant struct {
	ID           string
	TenantID     string
	Name         string
	ChannelIDs   []string // ids of the ChannelConnections this assistant serves
	ISPProfileID string   // optional providerhub ISP profile id ("" = no ISP tools)
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

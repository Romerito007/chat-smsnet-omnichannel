package entity

import "time"

// Assistant is a named copilot assistant a tenant can define (many per tenant).
// It reuses the tenant's AIConfig for provider/key/policies and adds routing: the
// channel types it serves and an optional pinned ISP profile. With an ISP profile
// the backend exposes the SMSNET tools to the model and injects the ISP config
// server-side; without one, no ISP tools are offered.
type Assistant struct {
	ID           string
	TenantID     string
	Name         string
	ChannelTypes []string // conversation Channel values this assistant serves (e.g. "whatsapp")
	ISPProfileID string   // optional providerhub ISP profile id ("" = no ISP tools)
	Enabled      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ServesChannel reports whether the assistant serves the given channel type.
func (a *Assistant) ServesChannel(channelType string) bool {
	for _, c := range a.ChannelTypes {
		if c == channelType {
			return true
		}
	}
	return false
}

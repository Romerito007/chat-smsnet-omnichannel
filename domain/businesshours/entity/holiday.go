package entity

import "time"

// HolidayScope describes which channels a holiday applies to.
type HolidayScope string

const (
	// ScopeAllChannels applies the holiday to every channel in the tenant.
	ScopeAllChannels HolidayScope = "all_channels"
	// ScopeChannels applies the holiday only to the listed channels.
	ScopeChannels HolidayScope = "channels"
)

// Holiday is a tenant day-off that closes business hours for the day. Date is a
// civil date "YYYY-MM-DD"; when Recurring is true only the month and day matter
// (the holiday repeats every year). A holiday is scoped to specific channels (the
// ChannelConnection that carries the conversation's business hours), or to all of
// them.
type Holiday struct {
	ID         string
	TenantID   string
	Date       string
	Name       string
	Scope      HolidayScope
	ChannelIDs []string
	Recurring  bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// AppliesTo reports whether the holiday governs the given channel.
func (h *Holiday) AppliesTo(channelID string) bool {
	if h.Scope == ScopeAllChannels {
		return true
	}
	for _, id := range h.ChannelIDs {
		if id == channelID {
			return true
		}
	}
	return false
}

// FallsOn reports whether the holiday covers the given local date. A recurring
// holiday matches on month+day; a one-off matches the exact date.
func (h *Holiday) FallsOn(local time.Time) bool {
	y, m, d := local.Date()
	hy, hm, hd, ok := splitDate(h.Date)
	if !ok {
		return false
	}
	if h.Recurring {
		return int(m) == hm && d == hd
	}
	return y == hy && int(m) == hm && d == hd
}

// splitDate parses "YYYY-MM-DD" into its parts.
func splitDate(date string) (year, month, day int, ok bool) {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return 0, 0, 0, false
	}
	yy, mm, dd := t.Date()
	return yy, int(mm), dd, true
}

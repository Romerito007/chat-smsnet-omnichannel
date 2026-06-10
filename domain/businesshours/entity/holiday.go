package entity

import "time"

// HolidayScope describes which sectors a holiday applies to.
type HolidayScope string

const (
	// ScopeAllSectors applies the holiday to every sector in the tenant.
	ScopeAllSectors HolidayScope = "all_sectors"
	// ScopeSectors applies the holiday only to the listed sectors.
	ScopeSectors HolidayScope = "sectors"
)

// Holiday is a tenant day-off that closes business hours for the day. Date is a
// civil date "YYYY-MM-DD"; when Recurring is true only the month and day matter
// (the holiday repeats every year).
type Holiday struct {
	ID        string
	TenantID  string
	Date      string
	Name      string
	Scope     HolidayScope
	SectorIDs []string
	Recurring bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// AppliesTo reports whether the holiday governs the given sector.
func (h *Holiday) AppliesTo(sectorID string) bool {
	if h.Scope == ScopeAllSectors {
		return true
	}
	for _, id := range h.SectorIDs {
		if id == sectorID {
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

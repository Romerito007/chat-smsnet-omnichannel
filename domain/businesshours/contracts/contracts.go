// Package contracts holds the businesshours service inputs and the
// business-status result.
package contracts

import "time"

// CreateHoliday is the input to create a holiday. When ChannelIDs is non-empty
// the holiday is scoped to those channels; otherwise it applies to all channels.
type CreateHoliday struct {
	Date       string
	Name       string
	ChannelIDs []string
	Recurring  *bool
}

// UpdateHoliday patches a holiday. Nil fields are left unchanged; a non-nil
// ChannelIDs replaces the scope (empty slice → all channels).
type UpdateHoliday struct {
	Date       *string
	Name       *string
	ChannelIDs *[]string
	Recurring  *bool
}

// StatusReason explains a business-status result.
type StatusReason string

const (
	ReasonOpen         StatusReason = "open"
	ReasonOutsideHours StatusReason = "outside_hours"
	ReasonHoliday      StatusReason = "holiday"
	ReasonUnconfigured StatusReason = "unconfigured" // open: no hours set
)

// BusinessStatus is the result of GET /v1/channels/{id}/business-status.
type BusinessStatus struct {
	ChannelID   string       `json:"channel_id"`
	Open        bool         `json:"open"`
	Reason      StatusReason `json:"reason"`
	Timezone    string       `json:"timezone"`
	LocalTime   time.Time    `json:"local_time"`
	HolidayName string       `json:"holiday_name,omitempty"`
	// TodayIntervals are the configured open intervals for the local weekday,
	// formatted "HH:MM-HH:MM".
	TodayIntervals []string `json:"today_intervals,omitempty"`
}

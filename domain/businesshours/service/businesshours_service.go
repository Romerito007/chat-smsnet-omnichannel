// Package service holds the businesshours logic: holiday CRUD and the
// timezone/holiday-aware business-hours check consulted by routing/automation.
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/businesshours/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/businesshours/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/businesshours/repository"
	channelrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// BusinessHoursService answers whether a channel is open, considering its
// timezone, weekly schedule and the tenant's holidays.
type BusinessHoursService struct {
	channels channelrepo.ConnectionRepository
	holidays repository.HolidayRepository
	clock    shared.Clock
}

// NewBusinessHoursService builds the service.
func NewBusinessHoursService(channels channelrepo.ConnectionRepository, holidays repository.HolidayRepository, clock shared.Clock) *BusinessHoursService {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &BusinessHoursService{channels: channels, holidays: holidays, clock: clock}
}

// IsWithinBusinessHours reports whether the channel is open at the given instant.
// It is the port consulted by routing/automation.
func (s *BusinessHoursService) IsWithinBusinessHours(ctx context.Context, channelID string, at time.Time) (bool, error) {
	status, err := s.Status(ctx, channelID, at)
	if err != nil {
		return false, err
	}
	return status.Open, nil
}

// Status computes the full business status for a channel at an instant. "Open
// now?" is resolved in the CHANNEL's timezone (business_hours.timezone), not the
// server's.
func (s *BusinessHoursService) Status(ctx context.Context, channelID string, at time.Time) (contracts.BusinessStatus, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return contracts.BusinessStatus{}, err
	}
	conn, err := s.channels.FindByID(ctx, channelID)
	if err != nil {
		return contracts.BusinessStatus{}, err
	}

	sched := entity.ParseSchedule(conn.BusinessHours)
	loc, lerr := time.LoadLocation(sched.Timezone)
	if lerr != nil {
		loc = time.UTC
		sched.Timezone = "UTC"
	}
	local := at.In(loc)

	base := contracts.BusinessStatus{
		ChannelID:      channelID,
		Timezone:       sched.Timezone,
		LocalTime:      local,
		TodayIntervals: formatIntervals(sched.IntervalsOn(local.Weekday())),
	}

	// A holiday closes the whole day regardless of the weekly schedule. Phase 2
	// honors global (all-channels) holidays; channel-scoped holidays arrive in
	// Phase 3.
	holidays, err := s.holidays.ListAll(ctx)
	if err != nil {
		return contracts.BusinessStatus{}, err
	}
	for _, h := range holidays {
		if h.AppliesTo(channelID) && h.FallsOn(local) {
			base.Open = false
			base.Reason = contracts.ReasonHoliday
			base.HolidayName = h.Name
			return base, nil
		}
	}

	switch {
	case !sched.Configured:
		base.Open = true
		base.Reason = contracts.ReasonUnconfigured
	case sched.IsOpenAt(local):
		base.Open = true
		base.Reason = contracts.ReasonOpen
	default:
		base.Open = false
		base.Reason = contracts.ReasonOutsideHours
	}
	return base, nil
}

// StatusNow computes the status at the current instant.
func (s *BusinessHoursService) StatusNow(ctx context.Context, channelID string) (contracts.BusinessStatus, error) {
	return s.Status(ctx, channelID, s.clock.Now())
}

func formatIntervals(intervals []entity.Interval) []string {
	if len(intervals) == 0 {
		return nil
	}
	out := make([]string, 0, len(intervals))
	for _, iv := range intervals {
		out = append(out, fmt.Sprintf("%s-%s", hhmm(iv.StartMin), hhmm(iv.EndMin)))
	}
	return out
}

func hhmm(min int) string {
	return fmt.Sprintf("%02d:%02d", min/60, min%60)
}

var _ shared.BusinessHoursChecker = (*BusinessHoursService)(nil)

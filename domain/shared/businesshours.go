package shared

import (
	"context"
	"time"
)

// BusinessHoursChecker reports whether a sector is within its business hours at a
// given instant, considering the sector's timezone and holidays. It is
// implemented by the businesshours domain and consulted by routing/automation;
// the default no-op treats everything as always open so the check never blocks
// when unwired.
type BusinessHoursChecker interface {
	IsWithinBusinessHours(ctx context.Context, sectorID string, at time.Time) (bool, error)
}

// NoopBusinessHoursChecker always reports "open".
type NoopBusinessHoursChecker struct{}

// IsWithinBusinessHours implements BusinessHoursChecker.
func (NoopBusinessHoursChecker) IsWithinBusinessHours(context.Context, string, time.Time) (bool, error) {
	return true, nil
}

// BusinessClock does business-time arithmetic for a sector, honoring its
// timezone, weekly schedule and holidays. It is implemented by the businesshours
// domain and consulted by the SLA domain to compute due dates "em horário útil".
// The default no-op treats time as 24/7 (wall-clock).
type BusinessClock interface {
	// AddBusinessDuration advances `from` by d of open business time, returning
	// the resulting instant. With no business hours configured it is from+d.
	AddBusinessDuration(ctx context.Context, sectorID string, from time.Time, d time.Duration) (time.Time, error)
	// BusinessDurationBetween returns the amount of open business time in
	// [from, to]. With no business hours configured it is to-from.
	BusinessDurationBetween(ctx context.Context, sectorID string, from, to time.Time) (time.Duration, error)
}

// NoopBusinessClock treats all time as business time (wall-clock 24/7).
type NoopBusinessClock struct{}

// AddBusinessDuration implements BusinessClock.
func (NoopBusinessClock) AddBusinessDuration(_ context.Context, _ string, from time.Time, d time.Duration) (time.Time, error) {
	return from.Add(d), nil
}

// BusinessDurationBetween implements BusinessClock.
func (NoopBusinessClock) BusinessDurationBetween(_ context.Context, _ string, from, to time.Time) (time.Duration, error) {
	if to.Before(from) {
		return 0, nil
	}
	return to.Sub(from), nil
}

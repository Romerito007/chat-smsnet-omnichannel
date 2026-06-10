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

package shared

import "time"

// Clock abstracts the current time so services are deterministic under test.
type Clock interface {
	Now() time.Time
}

// SystemClock returns the real wall-clock time in UTC.
type SystemClock struct{}

// Now implements Clock.
func (SystemClock) Now() time.Time { return time.Now().UTC() }

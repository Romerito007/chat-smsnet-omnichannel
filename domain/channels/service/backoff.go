package service

import "time"

// backoffSeconds returns an exponential backoff in seconds for a retry attempt,
// capped at 5 minutes.
func backoffSeconds(attempt int) int {
	if attempt < 1 {
		attempt = 1
	}
	v := 1 << attempt // 2,4,8,16,32...
	if v > 300 {
		v = 300
	}
	return v
}

func durationSeconds(s int) time.Duration {
	return time.Duration(s) * time.Second
}

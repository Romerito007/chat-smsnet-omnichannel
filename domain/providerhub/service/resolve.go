package service

const defaultTimeoutMs = 8000

// summarize trims an error to a short, body-free summary.
func summarize(err error) string {
	msg := err.Error()
	if len(msg) > 200 {
		msg = msg[:200]
	}
	return msg
}

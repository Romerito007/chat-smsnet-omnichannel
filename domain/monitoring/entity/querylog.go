package entity

import "time"

// QueryType identifies a monitoring gateway operation.
type QueryType string

const (
	QuerySummary   QueryType = "summary"
	QueryIncidents QueryType = "incidents"
	QueryTest      QueryType = "test"
)

// QueryStatus is the outcome of a query.
type QueryStatus string

const (
	StatusSuccess QueryStatus = "success"
	StatusError   QueryStatus = "error"
	StatusTimeout QueryStatus = "timeout"
	StatusBlocked QueryStatus = "blocked" // rate-limited
)

// MonitoringQueryLog is the minimal technical record of one on-demand query. It
// stores NO response body — only metadata for auditing/diagnostics.
type MonitoringQueryLog struct {
	ID             string
	TenantID       string
	UserID         string
	ContactID      string
	ConversationID string
	QueryType      QueryType
	Status         QueryStatus
	LatencyMs      int64
	ErrorSummary   string
	CreatedAt      time.Time
}

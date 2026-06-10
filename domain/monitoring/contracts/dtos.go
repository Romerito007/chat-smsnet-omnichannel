// Package contracts holds the normalized monitoring DTOs, the gateway port and
// the service inputs. The DTOs are returned to clients on demand and are NEVER
// persisted; metadata is filtered to a safe allow-list.
package contracts

import "time"

// Lookup identifies a customer to the external monitoring system. The service
// builds it from the conversation's contact.
type Lookup struct {
	ContactID  string
	Document   string
	Phone      string
	ExternalID string
}

// CustomerStatus is the normalized link state.
type CustomerStatus string

const (
	StatusOnline  CustomerStatus = "online"
	StatusOffline CustomerStatus = "offline"
	StatusUnknown CustomerStatus = "unknown"
)

// Severity is the normalized incident severity.
type Severity string

const (
	SeverityNormal   Severity = "normal"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// MonitoringSummary is the normalized technical status shown in the conversation
// tab. Metadata is a filtered, safe subset (no raw external payload).
type MonitoringSummary struct {
	CustomerStatus  CustomerStatus `json:"customer_status"`
	Severity        Severity       `json:"severity"`
	ActiveIncidents int            `json:"active_incidents"`
	LastDownAt      *time.Time     `json:"last_down_at,omitempty"`
	LastUpAt        *time.Time     `json:"last_up_at,omitempty"`
	Region          string         `json:"region,omitempty"`
	Device          string         `json:"device,omitempty"`
	Message         string         `json:"message,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

// Incident is a normalized monitoring incident.
type Incident struct {
	ID         string     `json:"id"`
	Severity   Severity   `json:"severity,omitempty"`
	Status     string     `json:"status,omitempty"`
	Title      string     `json:"title,omitempty"`
	Region     string     `json:"region,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

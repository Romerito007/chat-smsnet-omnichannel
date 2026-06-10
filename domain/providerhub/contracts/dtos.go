// Package contracts holds the normalized provider DTOs, the gateway port and the
// service inputs. These DTOs are returned to clients on demand and are NEVER
// persisted.
package contracts

import "time"

// Lookup identifies a customer to the standardized provider API. The service
// builds it from the conversation's contact.
type Lookup struct {
	ContactID  string
	Document   string
	Phone      string
	ExternalID string
}

// CustomerProfile is the normalized customer record.
type CustomerProfile struct {
	ProviderRef string         `json:"provider_ref,omitempty"`
	Name        string         `json:"name,omitempty"`
	Document    string         `json:"document,omitempty"`
	Email       string         `json:"email,omitempty"`
	Phone       string         `json:"phone,omitempty"`
	Address     string         `json:"address,omitempty"`
	Status      string         `json:"status,omitempty"`
	Extra       map[string]any `json:"extra,omitempty"`
}

// Contract is a normalized service contract.
type Contract struct {
	ID           string     `json:"id"`
	Plan         string     `json:"plan,omitempty"`
	Status       string     `json:"status,omitempty"`
	MonthlyValue float64    `json:"monthly_value,omitempty"`
	StartDate    *time.Time `json:"start_date,omitempty"`
}

// FinancialStatus is the normalized financial snapshot.
type FinancialStatus struct {
	Balance      float64   `json:"balance"`
	Overdue      bool      `json:"overdue"`
	OverdueValue float64   `json:"overdue_value,omitempty"`
	Invoices     []Invoice `json:"invoices,omitempty"`
}

// Invoice is a normalized invoice line.
type Invoice struct {
	ID      string     `json:"id"`
	Value   float64    `json:"value"`
	Status  string     `json:"status,omitempty"`
	DueDate *time.Time `json:"due_date,omitempty"`
}

// ConnectionStatus is the normalized connection/link status.
type ConnectionStatus struct {
	Online     bool       `json:"online"`
	IP         string     `json:"ip,omitempty"`
	Technology string     `json:"technology,omitempty"`
	SignalDBm  float64    `json:"signal_dbm,omitempty"`
	LastSeenAt *time.Time `json:"last_seen_at,omitempty"`
}

// Ticket is a normalized support ticket.
type Ticket struct {
	ID        string     `json:"id"`
	Subject   string     `json:"subject,omitempty"`
	Status    string     `json:"status,omitempty"`
	Priority  string     `json:"priority,omitempty"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
}

// OpenTicketInput is the payload to open a ticket.
type OpenTicketInput struct {
	Subject     string
	Description string
	Priority    string
}

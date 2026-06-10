package entity

import "time"

// QueryType identifies a provider gateway operation.
type QueryType string

const (
	QueryCustomerProfile  QueryType = "customer_profile"
	QueryContracts        QueryType = "contracts"
	QueryFinancialStatus  QueryType = "financial_status"
	QueryConnectionStatus QueryType = "connection_status"
	QueryTickets          QueryType = "tickets"
	QueryOpenTicket       QueryType = "open_ticket"
	QueryTest             QueryType = "test"
)

// QueryStatus is the outcome of a query.
type QueryStatus string

const (
	StatusSuccess QueryStatus = "success"
	StatusError   QueryStatus = "error"
	StatusTimeout QueryStatus = "timeout"
	StatusBlocked QueryStatus = "blocked" // rate-limited
)

// ProviderQueryLog is the minimal technical record of one on-demand query. It
// deliberately stores NO response body — only metadata for auditing/diagnostics.
type ProviderQueryLog struct {
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

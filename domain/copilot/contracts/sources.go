package contracts

import "context"

// CustomerDataSource provides the customer profile for a contact. Optional: a
// nil source means no customer enrichment is available. The service only calls
// it when the tenant's allow_customer_data policy permits.
type CustomerDataSource interface {
	Customer(ctx context.Context, contactID string) (*CustomerInfo, error)
}

// FinancialDataSource provides a financial summary for a contact. Optional and
// only consulted when allow_financial_data permits.
type FinancialDataSource interface {
	Financial(ctx context.Context, contactID string) (*FinancialInfo, error)
}

// MonitoringDataSource provides a technical-status summary for a conversation.
// Optional and only consulted when allow_monitoring_data permits.
type MonitoringDataSource interface {
	Monitoring(ctx context.Context, conversationID string) (*MonitoringInfo, error)
}

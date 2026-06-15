package contracts

import "context"

// CustomerDataSource provides the customer profile for a contact. Optional: a
// nil source means no customer enrichment is available. The service only calls
// it when the assistant's allow_customer_data gate permits.
type CustomerDataSource interface {
	Customer(ctx context.Context, contactID string) (*CustomerInfo, error)
}

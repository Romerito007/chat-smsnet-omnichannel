// Package entity holds the product catalog aggregate: a per-tenant product (name,
// price, sku) that can be added as a line item to a deal.
package entity

import "time"

// DefaultCurrency is used when a product is created without one.
const DefaultCurrency = "BRL"

// Product is a catalog product for a tenant. Deactivating (active=false) is preferred
// over deleting so deals that snapshot a product keep referencing a real id.
type Product struct {
	ID          string
	TenantID    string
	Name        string
	Description string
	Price       float64
	Currency    string
	SKU         string
	Active      bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

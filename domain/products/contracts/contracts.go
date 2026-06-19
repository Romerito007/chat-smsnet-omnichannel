// Package contracts holds the product-catalog service inputs and the module-gate port.
package contracts

import "context"

// CreateProduct is the input to create a catalog product.
type CreateProduct struct {
	Name        string
	Description string
	Price       float64
	Currency    string
	SKU         string
}

// UpdateProduct edits the editable fields. Nil = unchanged.
type UpdateProduct struct {
	Name        *string
	Description *string
	Price       *float64
	Currency    *string
	SKU         *string
	Active      *bool
}

// ListFilter narrows the catalog listing. Q is a name search; Active filters by the
// active flag (nil = both).
type ListFilter struct {
	Q      string
	Active *bool
}

// ModuleGate reports whether the products module is enabled for the tenant.
// Implemented over the crmsettings service. Optional: a nil gate means always enabled.
type ModuleGate interface {
	ProductsEnabled(ctx context.Context) (bool, error)
}

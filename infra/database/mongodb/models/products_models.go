package models

// Product is the BSON document for a tenant's catalog product.
type Product struct {
	Base        `bson:",inline"`
	Name        string  `bson:"name"`
	Description string  `bson:"description,omitempty"`
	Price       float64 `bson:"price"`
	Currency    string  `bson:"currency"`
	SKU         string  `bson:"sku,omitempty"`
	Active      bool    `bson:"active"`
}

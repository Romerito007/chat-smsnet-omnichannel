// Package products holds the request/response DTOs for the product-catalog endpoints.
package products

import (
	"time"

	ccontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/products/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/products/entity"
)

// CreateProductRequest is the body of POST /v1/crm/products.
type CreateProductRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	Currency    string  `json:"currency"`
	SKU         string  `json:"sku"`
}

// ToCommand maps the request to the service command.
func (r CreateProductRequest) ToCommand() ccontracts.CreateProduct {
	return ccontracts.CreateProduct{Name: r.Name, Description: r.Description, Price: r.Price, Currency: r.Currency, SKU: r.SKU}
}

// UpdateProductRequest is the body of PATCH /v1/crm/products/{id}. Nil = unchanged.
// Set active=false to deactivate (preferred over deleting).
type UpdateProductRequest struct {
	Name        *string  `json:"name"`
	Description *string  `json:"description"`
	Price       *float64 `json:"price"`
	Currency    *string  `json:"currency"`
	SKU         *string  `json:"sku"`
	Active      *bool    `json:"active"`
}

// ToCommand maps to the service command.
func (r UpdateProductRequest) ToCommand() ccontracts.UpdateProduct {
	return ccontracts.UpdateProduct{Name: r.Name, Description: r.Description, Price: r.Price, Currency: r.Currency, SKU: r.SKU, Active: r.Active}
}

// ProductResponse is the public representation of a catalog product.
type ProductResponse struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Price       float64   `json:"price"`
	Currency    string    `json:"currency"`
	SKU         string    `json:"sku,omitempty"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// NewProductResponse maps the entity to the DTO.
func NewProductResponse(p *entity.Product) ProductResponse {
	return ProductResponse{
		ID: p.ID, TenantID: p.TenantID, Name: p.Name, Description: p.Description,
		Price: p.Price, Currency: p.Currency, SKU: p.SKU, Active: p.Active,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

// NewProductResponses maps a slice.
func NewProductResponses(items []*entity.Product) []ProductResponse {
	out := make([]ProductResponse, len(items))
	for i, p := range items {
		out[i] = NewProductResponse(p)
	}
	return out
}

// Package repository declares the product-catalog persistence contract.
package repository

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/products/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/products/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// ProductRepository persists catalog products within a tenant (scope from ctx).
type ProductRepository interface {
	Create(ctx context.Context, p *entity.Product) error
	Update(ctx context.Context, p *entity.Product) error
	FindByID(ctx context.Context, id string) (*entity.Product, error)
	// List returns a keyset page of the tenant's products matching the filter.
	List(ctx context.Context, f contracts.ListFilter, page shared.PageRequest) ([]*entity.Product, error)
}

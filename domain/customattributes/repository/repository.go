// Package repository declares the custom-attribute definition persistence
// contract. Every method is tenant-scoped via the context.
package repository

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/customattributes/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// DefinitionRepository persists custom-attribute definitions.
type DefinitionRepository interface {
	Create(ctx context.Context, d *entity.Definition) error
	Update(ctx context.Context, d *entity.Definition) error
	Delete(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (*entity.Definition, error)
	// FindByKey returns the definition with the given (applies_to, key), or a
	// not_found AppError. Used to enforce uniqueness on create.
	FindByKey(ctx context.Context, appliesTo entity.AppliesTo, key string) (*entity.Definition, error)
	// List returns a page of definitions, optionally filtered by applies_to.
	List(ctx context.Context, appliesTo entity.AppliesTo, page shared.PageRequest) ([]*entity.Definition, error)
	// ListAllByAppliesTo returns every definition for a scope (the set is small),
	// used by the value validator.
	ListAllByAppliesTo(ctx context.Context, appliesTo entity.AppliesTo) ([]*entity.Definition, error)
}

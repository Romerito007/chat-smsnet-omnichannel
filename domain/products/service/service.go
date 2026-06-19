// Package service holds the product-catalog business logic: CRUD scoped by tenant and
// gated by the tenant's products toggle. Deactivating is preferred over deleting.
package service

import (
	"context"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/products/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/products/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/products/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// Service manages the product catalog.
type Service struct {
	repo    repository.ProductRepository
	gate    contracts.ModuleGate
	auditor shared.Auditor
	clock   shared.Clock
}

// New builds the service.
func New(repo repository.ProductRepository, clock shared.Clock) *Service {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &Service{repo: repo, auditor: shared.NoopAuditor{}, clock: clock}
}

// SetModuleGate wires the products on/off toggle (crmsettings). Optional.
func (s *Service) SetModuleGate(g contracts.ModuleGate) {
	if g != nil {
		s.gate = g
	}
}

// SetAuditor wires the audit trail (optional).
func (s *Service) SetAuditor(a shared.Auditor) {
	if a != nil {
		s.auditor = a
	}
}

// Create stores a new product.
func (s *Service) Create(ctx context.Context, cmd contracts.CreateProduct) (*entity.Product, error) {
	tenantID, err := s.guardWrite(ctx)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return nil, apperror.Validation("name is required")
	}
	if cmd.Price < 0 {
		return nil, apperror.Validation("price must be >= 0")
	}
	now := s.clock.Now()
	p := &entity.Product{
		ID: shared.NewID(), TenantID: tenantID, Name: name,
		Description: strings.TrimSpace(cmd.Description), Price: cmd.Price,
		Currency: currencyOr(cmd.Currency), SKU: strings.TrimSpace(cmd.SKU),
		Active: true, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	s.audit(ctx, "product.created", p.ID)
	return p, nil
}

// Update edits the editable fields (including active for deactivation).
func (s *Service) Update(ctx context.Context, id string, cmd contracts.UpdateProduct) (*entity.Product, error) {
	if _, err := s.guardWrite(ctx); err != nil {
		return nil, err
	}
	p, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if cmd.Name != nil {
		if strings.TrimSpace(*cmd.Name) == "" {
			return nil, apperror.Validation("name is required")
		}
		p.Name = strings.TrimSpace(*cmd.Name)
	}
	if cmd.Description != nil {
		p.Description = strings.TrimSpace(*cmd.Description)
	}
	if cmd.Price != nil {
		if *cmd.Price < 0 {
			return nil, apperror.Validation("price must be >= 0")
		}
		p.Price = *cmd.Price
	}
	if cmd.Currency != nil {
		p.Currency = currencyOr(*cmd.Currency)
	}
	if cmd.SKU != nil {
		p.SKU = strings.TrimSpace(*cmd.SKU)
	}
	if cmd.Active != nil {
		p.Active = *cmd.Active
	}
	p.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, p); err != nil {
		return nil, err
	}
	s.audit(ctx, "product.updated", p.ID)
	return p, nil
}

// List returns the tenant's catalog matching the filter, or an empty list when the
// products module is disabled.
func (s *Service) List(ctx context.Context, f contracts.ListFilter, page shared.PageRequest) ([]*entity.Product, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	on, err := s.enabled(ctx)
	if err != nil {
		return nil, err
	}
	if !on {
		return []*entity.Product{}, nil
	}
	return s.repo.List(ctx, f, page.Normalize())
}

// ── helpers ──────────────────────────────────────────────────────────────────

func (s *Service) guardWrite(ctx context.Context) (string, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return "", err
	}
	on, err := s.enabled(ctx)
	if err != nil {
		return "", err
	}
	if !on {
		return "", apperror.Conflict("the products module is disabled for this tenant")
	}
	return tenantID, nil
}

func (s *Service) enabled(ctx context.Context) (bool, error) {
	if s.gate == nil {
		return true, nil
	}
	return s.gate.ProductsEnabled(ctx)
}

func (s *Service) audit(ctx context.Context, action, id string) {
	_ = s.auditor.Record(ctx, shared.AuditEntry{Action: action, ResourceType: "product", ResourceID: id})
}

func currencyOr(c string) string {
	c = strings.TrimSpace(c)
	if c == "" {
		return entity.DefaultCurrency
	}
	return c
}

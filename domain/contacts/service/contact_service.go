// Package service holds the contact business logic.
package service

import (
	"context"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// Service manages tenant contacts.
type Service struct {
	repo  repository.ContactRepository
	clock shared.Clock
}

// New builds the service.
func New(repo repository.ContactRepository, clock shared.Clock) *Service {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &Service{repo: repo, clock: clock}
}

// Get returns a contact by id.
func (s *Service) Get(ctx context.Context, id string) (*entity.Contact, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}

// List returns a page of contacts.
func (s *Service) List(ctx context.Context, page shared.PageRequest) ([]*entity.Contact, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, page.Normalize())
}

// UpsertFromInbound finds the contact by its channel identity (or creates it),
// updating the basic locally-provided fields. Normalization is minimal: trim
// strings and reduce the phone to digits. No provider data is fetched or stored.
func (s *Service) UpsertFromInbound(ctx context.Context, cmd contracts.UpsertFromInbound) (*entity.Contact, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	channel := strings.TrimSpace(cmd.Channel)
	externalID := strings.TrimSpace(cmd.ExternalID)
	if channel == "" || externalID == "" {
		return nil, apperror.Validation("channel and external id are required to identify a contact")
	}

	name := strings.TrimSpace(cmd.Name)
	phone := normalizePhone(cmd.Phone)
	document := strings.TrimSpace(cmd.Document)
	now := s.clock.Now()

	existing, err := s.repo.FindByChannelIdentity(ctx, channel, externalID)
	if err != nil && apperror.From(err).Code != apperror.CodeNotFound {
		return nil, err
	}
	if existing != nil {
		// Update only when new, non-empty values arrive; never wipe existing data.
		changed := false
		if name != "" && name != existing.Name {
			existing.Name = name
			changed = true
		}
		if phone != "" && phone != existing.Phone {
			existing.Phone = phone
			changed = true
		}
		if document != "" && document != existing.Document {
			existing.Document = document
			changed = true
		}
		if changed {
			existing.UpdatedAt = now
			if err := s.repo.Update(ctx, existing); err != nil {
				return nil, err
			}
		}
		return existing, nil
	}

	contact := &entity.Contact{
		ID:         shared.NewID(),
		TenantID:   tenantID,
		Name:       name,
		Phone:      phone,
		Document:   document,
		Identities: []entity.ChannelIdentity{{Channel: channel, ExternalID: externalID}},
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.repo.Create(ctx, contact); err != nil {
		return nil, err
	}
	return contact, nil
}

// normalizePhone keeps digits and a leading '+'.
func normalizePhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return ""
	}
	var b strings.Builder
	for i, r := range phone {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else if r == '+' && i == 0 {
			b.WriteRune(r)
		}
	}
	return b.String()
}

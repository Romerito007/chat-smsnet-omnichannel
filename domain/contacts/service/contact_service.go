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
	repo    repository.ContactRepository
	clock   shared.Clock
	auditor shared.Auditor
	tags    contracts.TagResolver
}

// New builds the service.
func New(repo repository.ContactRepository, clock shared.Clock) *Service {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &Service{repo: repo, clock: clock, auditor: shared.NoopAuditor{}}
}

// SetAuditor wires the audit trail. Optional: when unset, contact writes are not
// audited.
func (s *Service) SetAuditor(a shared.Auditor) {
	if a != nil {
		s.auditor = a
	}
}

// SetTagResolver wires the tag-catalog resolver so contact tags are normalized to
// canonical ids (catalog names -> ids; free-text labels kept). Optional.
func (s *Service) SetTagResolver(r contracts.TagResolver) {
	if r != nil {
		s.tags = r
	}
}

// normalizeTags cleans the list and resolves catalog tag names to ids (lenient:
// unknown free-text labels pass through), so contact.tags mirrors conversation.tags.
func (s *Service) normalizeTags(ctx context.Context, tags []string) []string {
	out := cleanList(tags)
	if s.tags == nil || len(out) == 0 {
		return out
	}
	if resolved, err := s.tags.ResolveTags(ctx, out, false); err == nil {
		return resolved
	}
	return out
}

// Get returns a contact by id.
func (s *Service) Get(ctx context.Context, id string) (*entity.Contact, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}

// List returns a page of contacts narrowed by filter: the free-text query plus
// the optional name/phone substring and tag-id field filters (combined with AND).
func (s *Service) List(ctx context.Context, filter contracts.ListFilter, page shared.PageRequest) ([]*entity.Contact, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, filter, page.Normalize())
}

// Create stores a new CRM contact. It deduplicates within the tenant by document
// and by any phone (returns a conflict) before inserting, and audits the write.
func (s *Service) Create(ctx context.Context, cmd contracts.CreateContact) (*entity.Contact, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return nil, apperror.Validation("name is required").WithDetails(map[string]any{"name": "is required"})
	}
	phones := normalizePhones(cmd.Phones)
	document := strings.TrimSpace(cmd.Document)
	email := normalizeEmail(cmd.Email)

	if err := s.checkDuplicate(ctx, "", document, phones); err != nil {
		return nil, err
	}

	now := s.clock.Now()
	contact := &entity.Contact{
		ID:         shared.NewID(),
		TenantID:   tenantID,
		Name:       name,
		Document:   document,
		Email:      email,
		Identities: toIdentities(cmd.ExternalIDs),
		Tags:       s.normalizeTags(ctx, cmd.Tags),
		Notes:      strings.TrimSpace(cmd.Notes),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	contact.SetPhones(phones)
	if err := s.repo.Create(ctx, contact); err != nil {
		return nil, err
	}
	_ = s.auditor.Record(ctx, shared.AuditEntry{
		Action: "contact.created", ResourceType: "contact", ResourceID: contact.ID,
		Data: map[string]any{"name": contact.Name},
	})
	return contact, nil
}

// Update applies the non-nil fields of cmd (partial update), re-checking dedup
// for the resulting document/phones, and audits the write.
func (s *Service) Update(ctx context.Context, id string, cmd contracts.UpdateContact) (*entity.Contact, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	contact, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if cmd.Name != nil {
		name := strings.TrimSpace(*cmd.Name)
		if name == "" {
			return nil, apperror.Validation("name cannot be empty")
		}
		contact.Name = name
	}
	if cmd.Phones != nil {
		contact.SetPhones(normalizePhones(*cmd.Phones))
	}
	if cmd.Document != nil {
		contact.Document = strings.TrimSpace(*cmd.Document)
	}
	if cmd.Email != nil {
		contact.Email = normalizeEmail(*cmd.Email)
	}
	if cmd.Tags != nil {
		contact.Tags = s.normalizeTags(ctx, *cmd.Tags)
	}
	if cmd.Notes != nil {
		contact.Notes = strings.TrimSpace(*cmd.Notes)
	}
	if cmd.ExternalIDs != nil {
		contact.Identities = toIdentities(*cmd.ExternalIDs)
	}

	// Re-check dedup against OTHER contacts for the (possibly changed) keys.
	if err := s.checkDuplicate(ctx, contact.ID, contact.Document, contact.Phones); err != nil {
		return nil, err
	}

	contact.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, contact); err != nil {
		return nil, err
	}
	_ = s.auditor.Record(ctx, shared.AuditEntry{
		Action: "contact.updated", ResourceType: "contact", ResourceID: contact.ID,
	})
	return contact, nil
}

// checkDuplicate returns a conflict when another contact (id != selfID) in the
// tenant already owns the document or one of the phones.
func (s *Service) checkDuplicate(ctx context.Context, selfID, document string, phones []string) error {
	if document != "" {
		other, err := s.repo.FindByDocument(ctx, document)
		if err == nil && other.ID != selfID {
			return apperror.Conflict("a contact with this document already exists")
		}
		if err != nil && !isNotFound(err) {
			return err
		}
	}
	for _, phone := range phones {
		other, err := s.repo.FindByPhone(ctx, phone)
		if err == nil && other.ID != selfID {
			return apperror.Conflict("a contact with this phone already exists")
		}
		if err != nil && !isNotFound(err) {
			return err
		}
	}
	return nil
}

func isNotFound(err error) bool { return apperror.From(err).Code == apperror.CodeNotFound }

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
		if phone != "" && !containsPhone(existing.Phones, phone) {
			existing.AddPhone(phone)
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
		Document:   document,
		Identities: []entity.ChannelIdentity{{Channel: channel, ExternalID: externalID}},
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if phone != "" {
		contact.SetPhones([]string{phone})
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

// normalizePhones normalizes each phone and drops empties/duplicates, preserving
// order (the first becomes the primary).
func normalizePhones(phones []string) []string {
	out := make([]string, 0, len(phones))
	seen := map[string]struct{}{}
	for _, p := range phones {
		n := normalizePhone(p)
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	return out
}

func containsPhone(phones []string, phone string) bool {
	for _, p := range phones {
		if p == phone {
			return true
		}
	}
	return false
}

// normalizeEmail trims and lowercases an email (no validation beyond trimming).
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// cleanList trims and drops empty entries, preserving order.
func cleanList(items []string) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		if t := strings.TrimSpace(it); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// toIdentities maps CRM external-id inputs to channel identities, dropping
// incomplete pairs.
func toIdentities(ids []contracts.ExternalIdentity) []entity.ChannelIdentity {
	out := make([]entity.ChannelIdentity, 0, len(ids))
	for _, id := range ids {
		ch := strings.TrimSpace(id.Channel)
		ex := strings.TrimSpace(id.ExternalID)
		if ch == "" || ex == "" {
			continue
		}
		out = append(out, entity.ChannelIdentity{Channel: ch, ExternalID: ex})
	}
	return out
}

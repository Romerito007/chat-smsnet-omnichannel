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
	repo       repository.ContactRepository
	clock      shared.Clock
	auditor    shared.Auditor
	tags       contracts.TagResolver
	avatars    contracts.AvatarValidator
	avatarURLs shared.AvatarURLResolver
	customAttr shared.CustomAttributeValidator
	webhooks   shared.WebhookEmitter
	logger     shared.Logger
}

// New builds the service.
func New(repo repository.ContactRepository, clock shared.Clock) *Service {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &Service{repo: repo, clock: clock, auditor: shared.NoopAuditor{}, customAttr: shared.NoopCustomAttributeValidator{}, webhooks: shared.NoopWebhookEmitter{}, logger: shared.NewLogger("")}
}

// SetWebhookEmitter wires the outbound webhook emitter so contact create/update
// fan out to the tenant's webhooks (contact_created / contact_updated). Optional.
func (s *Service) SetWebhookEmitter(e shared.WebhookEmitter) {
	if e != nil {
		s.webhooks = e
	}
}

// SetCustomAttributeValidator wires the validator for custom_attributes (against
// applies_to=contact definitions). Optional: when unset, values pass through.
func (s *Service) SetCustomAttributeValidator(v shared.CustomAttributeValidator) {
	if v != nil {
		s.customAttr = v
	}
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

// SetAvatarValidator wires the attachment validator used to verify an
// avatar_attachment_id (exists, same tenant, image, ready) before persisting it.
// Optional: when unset, the avatar id is stored as-is.
func (s *Service) SetAvatarValidator(a contracts.AvatarValidator) {
	if a != nil {
		s.avatars = a
	}
}

// SetAvatarURLResolver wires the resolver that turns avatar_attachment_ids into
// short-lived signed avatar URLs for the response payloads. Optional.
func (s *Service) SetAvatarURLResolver(r shared.AvatarURLResolver) {
	if r != nil {
		s.avatarURLs = r
	}
}

// AvatarURLs batch-resolves a set of avatar attachment ids to signed URLs, keyed
// by attachment id. Best-effort and nil-safe (returns nil when unwired).
func (s *Service) AvatarURLs(ctx context.Context, attachmentIDs []string) (map[string]string, error) {
	if s.avatarURLs == nil || len(attachmentIDs) == 0 {
		return nil, nil
	}
	return s.avatarURLs.SignedAvatarURLs(ctx, attachmentIDs)
}

// ContactCards resolves a set of CONTACT ids to their display cards (name +
// signed avatar URL), keyed by contact id, in two batch queries: load the
// contacts, then sign their avatars. Used by the conversation inbox to render the
// contact per row without a second round-trip. Best-effort and nil-safe.
func (s *Service) ContactCards(ctx context.Context, contactIDs []string) (map[string]shared.DisplayCard, error) {
	if len(contactIDs) == 0 {
		return nil, nil
	}
	contacts, err := s.repo.FindByIDs(ctx, contactIDs)
	if err != nil {
		return nil, err
	}
	avatarIDs := make([]string, 0, len(contacts))
	for _, c := range contacts {
		if c.AvatarAttachmentID != "" {
			avatarIDs = append(avatarIDs, c.AvatarAttachmentID)
		}
	}
	var urls map[string]string
	if s.avatarURLs != nil {
		urls, _ = s.avatarURLs.SignedAvatarURLs(ctx, avatarIDs) // best-effort: name still resolves
	}
	out := make(map[string]shared.DisplayCard, len(contacts))
	for _, c := range contacts {
		out[c.ID] = shared.DisplayCard{Name: c.Name, AvatarURL: urls[c.AvatarAttachmentID]}
	}
	return out, nil
}

// validateAvatar runs the avatar attachment validation when a validator is wired
// and a non-empty id is provided, merging any field error into v.
func (s *Service) validateAvatar(ctx context.Context, attachmentID string, v map[string]any) {
	if s.avatars == nil || strings.TrimSpace(attachmentID) == "" {
		return
	}
	if err := s.avatars.ValidateReadyImage(ctx, attachmentID); err != nil {
		mergeDetails(v, apperror.From(err).Details)
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
	// Validate + normalize all fields, collecting every field error into one 400.
	v := map[string]any{}
	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		v["name"] = "is required"
	}
	phones, perr := normalizePhonesValidated(cmd.Phones)
	mergeDetails(v, perr)
	document, docOK := NormalizeDocument(cmd.Document)
	if !docOK {
		v["document"] = "is not a valid CPF or CNPJ"
	}
	email, emailOK := normalizeEmailStrict(cmd.Email)
	if !emailOK {
		v["email"] = "is not a valid email"
	}
	identities, ierr := normalizeIdentitiesValidated(cmd.ExternalIDs)
	mergeDetails(v, ierr)
	avatarID := strings.TrimSpace(cmd.AvatarAttachmentID)
	s.validateAvatar(ctx, avatarID, v)
	if len(v) > 0 {
		return nil, apperror.Validation("invalid contact").WithDetails(v)
	}

	if err := s.checkDuplicate(ctx, "", document, phones); err != nil {
		return nil, err
	}
	if err := s.checkIdentityUnique(ctx, "", identities); err != nil {
		return nil, err
	}

	now := s.clock.Now()
	contact := &entity.Contact{
		ID:                 shared.NewID(),
		TenantID:           tenantID,
		Name:               name,
		Document:           document,
		Email:              email,
		Identities:         identities,
		Tags:               s.normalizeTags(ctx, cmd.Tags),
		Notes:              strings.TrimSpace(cmd.Notes),
		AvatarAttachmentID: avatarID,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	contact.SetPhones(phones)
	if err := s.repo.Create(ctx, contact); err != nil {
		return nil, err
	}
	_ = s.auditor.Record(ctx, shared.AuditEntry{
		Action: "contact.created", ResourceType: "contact", ResourceID: contact.ID,
		Data: map[string]any{"name": contact.Name},
	})
	s.webhooks.Emit(ctx, contact.TenantID, contracts.EventContactCreated, "", contracts.NewContactPayload(contact))
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

	// Validate every provided field first, collecting all errors into one 400,
	// and only mutate the contact once everything is valid.
	v := map[string]any{}
	var newPhones []string
	if cmd.Name != nil {
		if name := strings.TrimSpace(*cmd.Name); name == "" {
			v["name"] = "cannot be empty"
		} else {
			contact.Name = name
		}
	}
	if cmd.Phones != nil {
		phones, perr := normalizePhonesValidated(*cmd.Phones)
		mergeDetails(v, perr)
		newPhones = phones
	}
	if cmd.Document != nil {
		if doc, ok := NormalizeDocument(*cmd.Document); ok {
			contact.Document = doc
		} else {
			v["document"] = "is not a valid CPF or CNPJ"
		}
	}
	if cmd.Email != nil {
		if email, ok := normalizeEmailStrict(*cmd.Email); ok {
			contact.Email = email
		} else {
			v["email"] = "is not a valid email"
		}
	}
	if cmd.Tags != nil {
		contact.Tags = s.normalizeTags(ctx, *cmd.Tags)
	}
	if cmd.Notes != nil {
		contact.Notes = strings.TrimSpace(*cmd.Notes)
	}
	if cmd.ExternalIDs != nil {
		identities, ierr := normalizeIdentitiesValidated(*cmd.ExternalIDs)
		mergeDetails(v, ierr)
		contact.Identities = identities
	}
	if cmd.AvatarAttachmentID != nil {
		avatarID := strings.TrimSpace(*cmd.AvatarAttachmentID)
		s.validateAvatar(ctx, avatarID, v)
		contact.AvatarAttachmentID = avatarID
	}
	if cmd.CustomAttributes != nil {
		attrs := *cmd.CustomAttributes
		if err := s.customAttr.ValidateCustomAttributes(ctx, "contact", attrs); err != nil {
			if apperror.From(err).Code == apperror.CodeValidation {
				mergeDetails(v, apperror.From(err).Details)
			} else {
				return nil, err
			}
		} else {
			contact.CustomAttributes = attrs
		}
	}
	if len(v) > 0 {
		return nil, apperror.Validation("invalid contact").WithDetails(v)
	}
	if cmd.Phones != nil {
		contact.SetPhones(newPhones)
	}

	// Re-check dedup + identity uniqueness against OTHER contacts.
	if err := s.checkDuplicate(ctx, contact.ID, contact.Document, contact.Phones); err != nil {
		return nil, err
	}
	if err := s.checkIdentityUnique(ctx, contact.ID, contact.Identities); err != nil {
		return nil, err
	}

	contact.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, contact); err != nil {
		return nil, err
	}
	_ = s.auditor.Record(ctx, shared.AuditEntry{
		Action: "contact.updated", ResourceType: "contact", ResourceID: contact.ID,
	})
	s.webhooks.Emit(ctx, contact.TenantID, contracts.EventContactUpdated, "", contracts.NewContactPayload(contact))
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

// checkIdentityUnique returns a conflict when another contact (id != selfID) in
// the tenant already carries one of the given channel identities — so a (channel,
// external_id) pair never matches two contacts (which would split conversation
// routing).
func (s *Service) checkIdentityUnique(ctx context.Context, selfID string, ids []entity.ChannelIdentity) error {
	for _, id := range ids {
		other, err := s.repo.FindByChannelIdentity(ctx, id.Channel, id.ExternalID)
		if err == nil && other.ID != selfID {
			return apperror.Conflict("a contact with this channel identity already exists").
				WithDetails(map[string]any{"external_ids": id.Channel + ":" + id.ExternalID + " already in use"})
		}
		if err != nil && !isNotFound(err) {
			return err
		}
	}
	return nil
}

// mergeDetails copies all entries of src into dst (field-error accumulation).
func mergeDetails(dst, src map[string]any) {
	for k, v := range src {
		dst[k] = v
	}
}

func isNotFound(err error) bool { return apperror.From(err).Code == apperror.CodeNotFound }

// AddChannelIdentity ensures a contact carries the given (channel, external_id)
// identity. It is called by a channel INTEGRATION (the WhatsApp gateway) over the
// inbound-token edge to persist a VERIFIED identifier (e.g. a resolved JID) back
// onto the contact, so later webhooks carry source=identity and skip re-verifying
// the phone. The tenant comes from the context (the inbound token), never a header.
//
// It is idempotent and additive: it never overwrites other identities, never errors
// on a repeat, and — to avoid splitting conversation routing — never STEALS an
// identity already owned by another contact (it logs and leaves it). applied is
// true only when a NEW identity was actually added.
func (s *Service) AddChannelIdentity(ctx context.Context, contactID, channel, externalID string) (applied bool, err error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return false, err
	}
	ch := strings.ToLower(strings.TrimSpace(channel))
	ex := strings.TrimSpace(externalID)
	if strings.TrimSpace(contactID) == "" {
		return false, apperror.Validation("contact_id is required")
	}
	if ex == "" {
		return false, apperror.Validation("external_id is required")
	}
	if ch == "" || !validIdentityChannel(ch) {
		return false, apperror.Validation("unsupported channel").
			WithDetails(map[string]any{"channel": "is not a supported channel"})
	}

	contact, err := s.repo.FindByID(ctx, contactID) // tenant-scoped: cross-tenant → not_found
	if err != nil {
		return false, err
	}
	if contact.HasIdentity(ch, ex) {
		return false, nil // already present — idempotent no-op
	}
	// Uniqueness: never steal an identity that already belongs to ANOTHER contact —
	// that would make one (channel, external_id) match two contacts and split routing.
	if other, ferr := s.repo.FindByChannelIdentity(ctx, ch, ex); ferr == nil {
		if other.ID != contact.ID {
			shared.LoggerFrom(ctx, s.logger).Warn("CONTACT_IDENTITY_IN_USE_BY_OTHER",
				"channel", ch, "contact_id", contact.ID, "owner_contact_id", other.ID)
			return false, nil // logged; not applied (not an error — the gateway must not fail)
		}
	} else if !isNotFound(ferr) {
		return false, ferr
	}

	contact.Identities = append(contact.Identities, entity.ChannelIdentity{Channel: ch, ExternalID: ex})
	contact.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, contact); err != nil {
		return false, err
	}
	// Audit without PII: record the channel, not the external id (the JID).
	_ = s.auditor.Record(ctx, shared.AuditEntry{
		Action: "contact.identity_updated", ResourceType: "contact", ResourceID: contact.ID,
		Data: map[string]any{"channel": ch},
	})
	return true, nil
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
			s.webhooks.Emit(ctx, existing.TenantID, contracts.EventContactUpdated, "", contracts.NewContactPayload(existing))
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
	s.webhooks.Emit(ctx, contact.TenantID, contracts.EventContactCreated, "", contracts.NewContactPayload(contact))
	return contact, nil
}

// UpsertGroupContact resolves (or creates) the single contact that represents a
// WhatsApp GROUP, keyed by the group JID — NOT by a phone (a group has none). It is
// the Domain-2 counterpart of UpsertFromInbound: an inbound group message maps to ONE
// group contact (Kind=group), never to one contact per member. name/description come
// from the synced registry (whatsapp_groups). Idempotent and additive: an existing
// group contact only has its name refreshed when the registry name changed.
func (s *Service) UpsertGroupContact(ctx context.Context, channel, groupJID, name, description string) (*entity.Contact, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	channel = strings.TrimSpace(channel)
	groupJID = strings.TrimSpace(groupJID)
	if channel == "" || groupJID == "" {
		return nil, apperror.Validation("channel and group_jid are required to identify a group contact")
	}
	name = strings.TrimSpace(name)
	now := s.clock.Now()

	existing, err := s.repo.FindByChannelIdentity(ctx, channel, groupJID)
	if err != nil && apperror.From(err).Code != apperror.CodeNotFound {
		return nil, err
	}
	if existing != nil {
		changed := false
		// Heal a record created before the group kind existed (or via another path).
		if existing.Kind != entity.KindGroup {
			existing.Kind = entity.KindGroup
			changed = true
		}
		if name != "" && name != existing.Name {
			existing.Name = name
			changed = true
		}
		if changed {
			existing.UpdatedAt = now
			if err := s.repo.Update(ctx, existing); err != nil {
				return nil, err
			}
			s.webhooks.Emit(ctx, existing.TenantID, contracts.EventContactUpdated, "", contracts.NewContactPayload(existing))
		}
		return existing, nil
	}

	contact := &entity.Contact{
		ID:         shared.NewID(),
		TenantID:   tenantID,
		Kind:       entity.KindGroup,
		Name:       name,
		Notes:      strings.TrimSpace(description), // seeded once from the registry
		Identities: []entity.ChannelIdentity{{Channel: channel, ExternalID: groupJID}},
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.repo.Create(ctx, contact); err != nil {
		return nil, err
	}
	s.webhooks.Emit(ctx, contact.TenantID, contracts.EventContactCreated, "", contracts.NewContactPayload(contact))
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

func containsPhone(phones []string, phone string) bool {
	for _, p := range phones {
		if p == phone {
			return true
		}
	}
	return false
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

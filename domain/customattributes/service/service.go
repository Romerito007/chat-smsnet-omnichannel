// Package service holds the custom-attribute definition logic: CRUD plus the
// value validator consulted by the contacts and conversations services.
package service

import (
	"context"
	"regexp"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/customattributes/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/customattributes/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/customattributes/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// Service manages custom-attribute definitions and validates values against them.
type Service struct {
	repo  repository.DefinitionRepository
	clock shared.Clock
}

// New builds the service.
func New(repo repository.DefinitionRepository, clock shared.Clock) *Service {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &Service{repo: repo, clock: clock}
}

// Create registers a definition. Key + applies_to + type are fixed here.
func (s *Service) Create(ctx context.Context, cmd contracts.CreateDefinition) (*entity.Definition, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	key := strings.TrimSpace(cmd.Key)
	label := strings.TrimSpace(cmd.Label)
	options := cleanOptions(cmd.Options)
	regex := strings.TrimSpace(cmd.Regex)

	v := map[string]any{}
	if key == "" {
		v["key"] = "is required"
	}
	if label == "" {
		v["label"] = "is required"
	}
	if !cmd.Type.Valid() {
		v["type"] = "must be one of text|number|boolean|date|list"
	}
	if !cmd.AppliesTo.Valid() {
		v["applies_to"] = "must be contact or conversation"
	}
	if cmd.Type == entity.TypeList && len(options) == 0 {
		v["options"] = "at least one option is required for a list attribute"
	}
	if cmd.Type != entity.TypeList && len(options) > 0 {
		v["options"] = "options are only valid for a list attribute"
	}
	if regex != "" {
		if cmd.Type != entity.TypeText {
			v["regex"] = "regex is only valid for a text attribute"
		} else if _, err := regexp.Compile(regex); err != nil {
			v["regex"] = "is not a valid regular expression"
		}
	}
	if len(v) > 0 {
		return nil, apperror.Validation("invalid definition").WithDetails(v)
	}

	// Uniqueness: one key per (tenant, applies_to).
	if _, err := s.repo.FindByKey(ctx, cmd.AppliesTo, key); err == nil {
		return nil, apperror.Conflict("a definition with this key already exists for this scope").
			WithDetails(map[string]any{"key": "already exists"})
	} else if apperror.From(err).Code != apperror.CodeNotFound {
		return nil, err
	}

	now := s.clock.Now()
	def := &entity.Definition{
		ID:          shared.NewID(),
		TenantID:    tenantID,
		Key:         key,
		Label:       label,
		Description: strings.TrimSpace(cmd.Description),
		Type:        cmd.Type,
		AppliesTo:   cmd.AppliesTo,
		Options:     options,
		Regex:       regex,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repo.Create(ctx, def); err != nil {
		return nil, err
	}
	return def, nil
}

// Update patches the editable fields (label/description/options/regex). Key,
// applies_to and type are immutable.
func (s *Service) Update(ctx context.Context, id string, cmd contracts.UpdateDefinition) (*entity.Definition, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	def, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	v := map[string]any{}
	if cmd.Label != nil {
		if label := strings.TrimSpace(*cmd.Label); label == "" {
			v["label"] = "cannot be empty"
		} else {
			def.Label = label
		}
	}
	if cmd.Description != nil {
		def.Description = strings.TrimSpace(*cmd.Description)
	}
	if cmd.Options != nil {
		options := cleanOptions(*cmd.Options)
		if def.Type == entity.TypeList && len(options) == 0 {
			v["options"] = "at least one option is required for a list attribute"
		} else if def.Type != entity.TypeList && len(options) > 0 {
			v["options"] = "options are only valid for a list attribute"
		} else {
			def.Options = options
		}
	}
	if cmd.Regex != nil {
		regex := strings.TrimSpace(*cmd.Regex)
		if regex != "" && def.Type != entity.TypeText {
			v["regex"] = "regex is only valid for a text attribute"
		} else if regex != "" {
			if _, err := regexp.Compile(regex); err != nil {
				v["regex"] = "is not a valid regular expression"
			} else {
				def.Regex = regex
			}
		} else {
			def.Regex = ""
		}
	}
	if len(v) > 0 {
		return nil, apperror.Validation("invalid definition").WithDetails(v)
	}

	def.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, def); err != nil {
		return nil, err
	}
	return def, nil
}

// Delete removes a definition. Existing values for the key become orphaned and are
// ignored on display (and auto-cleaned on the next write that omits them).
func (s *Service) Delete(ctx context.Context, id string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// Get returns a definition by id.
func (s *Service) Get(ctx context.Context, id string) (*entity.Definition, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}

// List returns a page of definitions, optionally filtered by applies_to (the
// Conversas/Contato tabs). An empty/invalid scope lists all.
func (s *Service) List(ctx context.Context, appliesTo entity.AppliesTo, page shared.PageRequest) ([]*entity.Definition, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, appliesTo, page.Normalize())
}

// ValidateCustomAttributes implements shared.CustomAttributeValidator: every key
// must have a definition for the given scope and every value must match its type.
// A nil/empty map is valid. Unknown keys and bad values are reported per key.
func (s *Service) ValidateCustomAttributes(ctx context.Context, appliesTo string, attrs map[string]any) error {
	if len(attrs) == 0 {
		return nil
	}
	scope := entity.AppliesTo(appliesTo)
	if !scope.Valid() {
		return apperror.Internal("invalid custom-attribute scope")
	}
	defs, err := s.repo.ListAllByAppliesTo(ctx, scope)
	if err != nil {
		return err
	}
	byKey := make(map[string]*entity.Definition, len(defs))
	for _, d := range defs {
		byKey[d.Key] = d
	}

	v := map[string]any{}
	for key, value := range attrs {
		def, ok := byKey[key]
		if !ok {
			v["custom_attributes."+key] = "unknown attribute for this scope"
			continue
		}
		if err := def.ValidateValue(value); err != nil {
			v["custom_attributes."+key] = err.Error()
		}
	}
	if len(v) > 0 {
		return apperror.Validation("invalid custom attributes").WithDetails(v)
	}
	return nil
}

// cleanOptions trims and de-dupes option labels, dropping empties.
func cleanOptions(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, o := range in {
		o = strings.TrimSpace(o)
		if o == "" || seen[o] {
			continue
		}
		seen[o] = true
		out = append(out, o)
	}
	return out
}

var _ shared.CustomAttributeValidator = (*Service)(nil)

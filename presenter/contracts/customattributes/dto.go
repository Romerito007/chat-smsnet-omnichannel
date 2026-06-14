// Package customattributes holds the request/response DTOs for custom-attribute
// definition management.
package customattributes

import (
	"time"

	cacontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/customattributes/contracts"
	caentity "github.com/romerito007/chat-smsnet-omnichannel/domain/customattributes/entity"
)

// CreateDefinitionRequest is the body of POST /v1/custom-attributes.
type CreateDefinitionRequest struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Type        string   `json:"type"`
	AppliesTo   string   `json:"applies_to"`
	Options     []string `json:"options"`
	Regex       string   `json:"regex"`
}

// ToCommand maps the request to the service command.
func (r CreateDefinitionRequest) ToCommand() cacontracts.CreateDefinition {
	return cacontracts.CreateDefinition{
		Key:         r.Key,
		Label:       r.Label,
		Description: r.Description,
		Type:        caentity.AttributeType(r.Type),
		AppliesTo:   caentity.AppliesTo(r.AppliesTo),
		Options:     r.Options,
		Regex:       r.Regex,
	}
}

// UpdateDefinitionRequest is the body of PATCH /v1/custom-attributes/{id}. Key,
// applies_to and type are immutable and not accepted here.
type UpdateDefinitionRequest struct {
	Label       *string   `json:"label"`
	Description *string   `json:"description"`
	Options     *[]string `json:"options"`
	Regex       *string   `json:"regex"`
}

// ToCommand maps the request to the service command.
func (r UpdateDefinitionRequest) ToCommand() cacontracts.UpdateDefinition {
	return cacontracts.UpdateDefinition{
		Label:       r.Label,
		Description: r.Description,
		Options:     r.Options,
		Regex:       r.Regex,
	}
}

// DefinitionResponse is the public representation of a definition.
type DefinitionResponse struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	Key         string    `json:"key"`
	Label       string    `json:"label"`
	Description string    `json:"description,omitempty"`
	Type        string    `json:"type"`
	AppliesTo   string    `json:"applies_to"`
	Options     []string  `json:"options,omitempty"`
	Regex       string    `json:"regex,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// NewDefinitionResponse maps a definition entity.
func NewDefinitionResponse(d *caentity.Definition) DefinitionResponse {
	return DefinitionResponse{
		ID: d.ID, TenantID: d.TenantID, Key: d.Key, Label: d.Label, Description: d.Description,
		Type: string(d.Type), AppliesTo: string(d.AppliesTo), Options: d.Options, Regex: d.Regex,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

// NewDefinitionResponses maps a slice.
func NewDefinitionResponses(items []*caentity.Definition) []DefinitionResponse {
	out := make([]DefinitionResponse, 0, len(items))
	for _, d := range items {
		out = append(out, NewDefinitionResponse(d))
	}
	return out
}

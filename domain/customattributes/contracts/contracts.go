// Package contracts holds the custom-attribute definition service inputs.
package contracts

import "github.com/romerito007/chat-smsnet-omnichannel/domain/customattributes/entity"

// CreateDefinition is the input to register a definition. Key and AppliesTo are
// fixed at creation.
type CreateDefinition struct {
	Key         string
	Label       string
	Description string
	Type        entity.AttributeType
	AppliesTo   entity.AppliesTo
	Options     []string
	Regex       string
}

// UpdateDefinition patches a definition. Key and AppliesTo and Type are immutable;
// only the display/validation fields are editable. Nil fields are left unchanged.
type UpdateDefinition struct {
	Label       *string
	Description *string
	Options     *[]string
	Regex       *string
}

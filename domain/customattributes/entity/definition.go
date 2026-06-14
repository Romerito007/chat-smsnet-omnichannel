// Package entity holds the custom-attribute definition aggregate. A definition is
// registered once per tenant and applies to either contacts or conversations; the
// values live on those entities (a key→value map), validated against definitions.
package entity

import (
	"fmt"
	"regexp"
	"time"
)

// AttributeType is the value kind a definition accepts.
type AttributeType string

const (
	TypeText    AttributeType = "text"
	TypeNumber  AttributeType = "number"
	TypeBoolean AttributeType = "boolean"
	TypeDate    AttributeType = "date" // civil date "YYYY-MM-DD"
	TypeList    AttributeType = "list" // value must be one of Options
)

// Valid reports whether t is a known type.
func (t AttributeType) Valid() bool {
	switch t {
	case TypeText, TypeNumber, TypeBoolean, TypeDate, TypeList:
		return true
	}
	return false
}

// AppliesTo is the entity a definition governs.
type AppliesTo string

const (
	AppliesToContact      AppliesTo = "contact"
	AppliesToConversation AppliesTo = "conversation"
)

// Valid reports whether a is a known scope.
func (a AppliesTo) Valid() bool {
	return a == AppliesToContact || a == AppliesToConversation
}

// Definition is a tenant's custom-attribute definition. Key is immutable after
// creation (it is the storage key in value maps); the rest is editable.
type Definition struct {
	ID          string
	TenantID    string
	Key         string
	Label       string
	Description string
	Type        AttributeType
	AppliesTo   AppliesTo
	Options     []string // required (non-empty) when Type == list
	Regex       string   // optional, only when Type == text
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ValidateValue checks a value against this definition's type/options/regex. The
// value is the JSON-decoded form (numbers arrive as float64).
func (d *Definition) ValidateValue(v any) error {
	switch d.Type {
	case TypeText:
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("must be a string")
		}
		if d.Regex != "" {
			matched, err := regexp.MatchString(d.Regex, s)
			if err != nil {
				return fmt.Errorf("definition has an invalid regex")
			}
			if !matched {
				return fmt.Errorf("does not match the required format")
			}
		}
	case TypeNumber:
		switch v.(type) {
		case float64, float32, int, int32, int64:
		default:
			return fmt.Errorf("must be a number")
		}
	case TypeBoolean:
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("must be a boolean")
		}
	case TypeDate:
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("must be a date string")
		}
		if _, err := time.Parse("2006-01-02", s); err != nil {
			return fmt.Errorf("must be a YYYY-MM-DD date")
		}
	case TypeList:
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("must be one of the allowed options")
		}
		for _, opt := range d.Options {
			if opt == s {
				return nil
			}
		}
		return fmt.Errorf("must be one of the allowed options")
	default:
		return fmt.Errorf("unknown attribute type")
	}
	return nil
}

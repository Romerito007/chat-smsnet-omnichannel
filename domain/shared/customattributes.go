package shared

import "context"

// CustomAttributeValidator validates a key→value custom-attributes map against the
// tenant's definitions for an entity scope ("contact" or "conversation"). It is
// the port consulted by the contacts and conversations services on write.
type CustomAttributeValidator interface {
	ValidateCustomAttributes(ctx context.Context, appliesTo string, attrs map[string]any) error
}

// NoopCustomAttributeValidator accepts any map (used when the validator is not
// wired, e.g. in tests).
type NoopCustomAttributeValidator struct{}

// ValidateCustomAttributes always passes.
func (NoopCustomAttributeValidator) ValidateCustomAttributes(context.Context, string, map[string]any) error {
	return nil
}

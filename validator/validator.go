// Package validator provides small, dependency-free validation helpers used by
// controllers to validate decoded DTOs, returning apperror.Validation on
// failure so the standard error envelope is produced.
package validator

import (
	"net/mail"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
)

// Errors accumulates field-level validation problems.
type Errors struct {
	fields map[string]any
}

// New returns an empty validation accumulator.
func New() *Errors { return &Errors{fields: map[string]any{}} }

// Require records an error when value is empty (after trimming).
func (e *Errors) Require(field, value string) *Errors {
	if strings.TrimSpace(value) == "" {
		e.fields[field] = "is required"
	}
	return e
}

// Email records an error when value is not a valid email address.
func (e *Errors) Email(field, value string) *Errors {
	if _, err := mail.ParseAddress(value); err != nil {
		e.fields[field] = "must be a valid email"
	}
	return e
}

// Check records an error with a custom message when ok is false.
func (e *Errors) Check(field string, ok bool, message string) *Errors {
	if !ok {
		e.fields[field] = message
	}
	return e
}

// Err returns a validation AppError when any field failed, or nil otherwise.
func (e *Errors) Err() error {
	if len(e.fields) == 0 {
		return nil
	}
	return apperror.Validation("validation failed").WithDetails(e.fields)
}

// Package apperror defines the canonical application error type and the
// standard error envelope used across the whole backend.
//
// Every error that reaches the HTTP/WS border is normalized into an AppError
// so the presenter layer can render the documented envelope:
//
//	{ "error": { "code", "message", "details", "request_id" } }
package apperror

import (
	"errors"
	"fmt"
	"net/http"
)

// Code is the machine-readable error code returned to clients.
type Code string

// Canonical error codes. These are part of the public API contract and must
// remain stable.
const (
	CodeValidation             Code = "validation_error"
	CodeUnauthorized           Code = "unauthorized"
	CodeForbidden              Code = "forbidden"
	CodeNotFound               Code = "not_found"
	CodeConflict               Code = "conflict"
	CodeRateLimited            Code = "rate_limited"
	CodeIntegrationUnavailable Code = "integration_unavailable"
	CodeInternal               Code = "internal_error"
)

// httpStatus maps each code to its default HTTP status.
var httpStatus = map[Code]int{
	CodeValidation:             http.StatusBadRequest,
	CodeUnauthorized:           http.StatusUnauthorized,
	CodeForbidden:              http.StatusForbidden,
	CodeNotFound:               http.StatusNotFound,
	CodeConflict:               http.StatusConflict,
	CodeRateLimited:            http.StatusTooManyRequests,
	CodeIntegrationUnavailable: http.StatusBadGateway,
	CodeInternal:               http.StatusInternalServerError,
}

// AppError is the single error type that flows through the layers. It carries a
// stable code, a human-readable message, optional structured details, and an
// optional wrapped cause (never exposed to the client).
type AppError struct {
	Code    Code           `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
	cause   error
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap exposes the wrapped cause for errors.Is / errors.As.
func (e *AppError) Unwrap() error { return e.cause }

// HTTPStatus returns the HTTP status associated with the error code.
func (e *AppError) HTTPStatus() int {
	if s, ok := httpStatus[e.Code]; ok {
		return s
	}
	return http.StatusInternalServerError
}

// WithDetails returns a copy of the error with the given structured details
// attached. Existing details are preserved unless overwritten by the same key.
func (e *AppError) WithDetails(details map[string]any) *AppError {
	clone := *e
	if clone.Details == nil {
		clone.Details = map[string]any{}
	} else {
		merged := make(map[string]any, len(clone.Details)+len(details))
		for k, v := range clone.Details {
			merged[k] = v
		}
		clone.Details = merged
	}
	for k, v := range details {
		clone.Details[k] = v
	}
	return &clone
}

// Wrap attaches an underlying cause to the error.
func (e *AppError) Wrap(cause error) *AppError {
	clone := *e
	clone.cause = cause
	return &clone
}

// New builds an AppError from a code and message.
func New(code Code, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

// Newf builds an AppError with a formatted message.
func Newf(code Code, format string, args ...any) *AppError {
	return &AppError{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Convenience constructors for the common cases.

func Validation(message string) *AppError   { return New(CodeValidation, message) }
func Unauthorized(message string) *AppError { return New(CodeUnauthorized, message) }
func Forbidden(message string) *AppError    { return New(CodeForbidden, message) }
func NotFound(message string) *AppError     { return New(CodeNotFound, message) }
func Conflict(message string) *AppError     { return New(CodeConflict, message) }
func RateLimited(message string) *AppError  { return New(CodeRateLimited, message) }
func Internal(message string) *AppError     { return New(CodeInternal, message) }
func Integration(message string) *AppError  { return New(CodeIntegrationUnavailable, message) }

// From normalizes any error into an AppError. If err is already an AppError it
// is returned unchanged; otherwise it is wrapped as an internal error so its
// cause is preserved for logging but hidden from clients.
func From(err error) *AppError {
	if err == nil {
		return nil
	}
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return Internal("internal server error").Wrap(err)
}

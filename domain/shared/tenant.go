package shared

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
)

// TenantID identifies a tenant. Every entity in the system is scoped by it.
type TenantID = string

// ctxKey is an unexported context key type to avoid collisions.
type ctxKey int

const (
	tenantKey ctxKey = iota
	actorKey
	requestIDKey
)

// Actor describes the authenticated principal making a request. It is the
// minimal identity the domain needs; richer authz data lives in domain/authz.
type Actor struct {
	UserID   ID
	TenantID TenantID
	Roles    []string
}

// WithTenant returns a child context carrying the tenant id.
func WithTenant(ctx context.Context, tenantID TenantID) context.Context {
	return context.WithValue(ctx, tenantKey, tenantID)
}

// TenantFrom extracts the tenant id from the context. The boolean reports
// whether a tenant was present.
func TenantFrom(ctx context.Context) (TenantID, bool) {
	v, ok := ctx.Value(tenantKey).(TenantID)
	return v, ok
}

// RequireTenant extracts the tenant id or returns a forbidden error when the
// request is not tenant-scoped. Repositories and services use this to enforce
// the "every entity respects tenant_id" invariant.
func RequireTenant(ctx context.Context) (TenantID, error) {
	if t, ok := TenantFrom(ctx); ok && t != "" {
		return t, nil
	}
	return "", apperror.Forbidden("missing tenant scope")
}

// WithActor returns a child context carrying the authenticated actor.
func WithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, actorKey, actor)
}

// ActorFrom extracts the authenticated actor from the context.
func ActorFrom(ctx context.Context) (Actor, bool) {
	v, ok := ctx.Value(actorKey).(Actor)
	return v, ok
}

// WithRequestID returns a child context carrying the request id used for
// tracing and error envelopes.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// RequestIDFrom extracts the request id from the context, returning "" when
// absent.
func RequestIDFrom(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

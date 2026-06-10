package authz

import "context"

// SectorScope controls how broadly a role's permissions apply across sectors.
type SectorScope string

const (
	// ScopeAll grants the role's permissions across every sector in the tenant.
	ScopeAll SectorScope = "all"
	// ScopeOwn restricts the role's permissions to the user's own sectors.
	ScopeOwn SectorScope = "own"
)

// Valid reports whether s is a known scope.
func (s SectorScope) Valid() bool {
	return s == ScopeAll || s == ScopeOwn
}

// AuthContext is the resolved identity + authorization state for a request. It
// is built by the AuthContext middleware from the verified access token and read
// by services to enforce tenant isolation, permissions and sector scope.
type AuthContext struct {
	TenantID    string
	UserID      string
	Permissions map[Permission]struct{}
	SectorIDs   []string
	SectorScope SectorScope
}

// NewAuthContext builds an AuthContext, normalizing the permission slice into a
// set for O(1) checks.
func NewAuthContext(tenantID, userID string, perms []Permission, sectorIDs []string, scope SectorScope) AuthContext {
	set := make(map[Permission]struct{}, len(perms))
	for _, p := range perms {
		set[p] = struct{}{}
	}
	if !scope.Valid() {
		scope = ScopeOwn
	}
	return AuthContext{
		TenantID:    tenantID,
		UserID:      userID,
		Permissions: set,
		SectorIDs:   sectorIDs,
		SectorScope: scope,
	}
}

// SystemActor returns an all-scope AuthContext for trusted internal/system
// operations (e.g. automation applying a flow decision). It is never created
// from external input.
func SystemActor(tenantID string) AuthContext {
	return NewAuthContext(tenantID, "system", AllPermissions(), nil, ScopeAll)
}

// Has reports whether the context holds the given permission.
func (a AuthContext) Has(p Permission) bool {
	_, ok := a.Permissions[p]
	return ok
}

// PermissionList returns the held permissions in catalog order (for /me, tokens).
func (a AuthContext) PermissionList() []Permission {
	out := make([]Permission, 0, len(a.Permissions))
	for _, p := range AllPermissions() {
		if _, ok := a.Permissions[p]; ok {
			out = append(out, p)
		}
	}
	return out
}

// CanAccessSector reports whether the actor may act within sectorID given its
// scope. ScopeAll always passes; ScopeOwn passes only for the actor's sectors.
func (a AuthContext) CanAccessSector(sectorID string) bool {
	if a.SectorScope == ScopeAll {
		return true
	}
	for _, s := range a.SectorIDs {
		if s == sectorID {
			return true
		}
	}
	return false
}

// ctxKey is unexported to avoid collisions.
type ctxKey int

const authCtxKey ctxKey = iota

// WithAuthContext stores the AuthContext on the context.
func WithAuthContext(ctx context.Context, ac AuthContext) context.Context {
	return context.WithValue(ctx, authCtxKey, ac)
}

// FromContext extracts the AuthContext; ok is false when the request is
// unauthenticated.
func FromContext(ctx context.Context) (AuthContext, bool) {
	ac, ok := ctx.Value(authCtxKey).(AuthContext)
	return ac, ok
}

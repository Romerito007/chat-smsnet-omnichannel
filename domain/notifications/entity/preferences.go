package entity

import "time"

// Preferences holds a user's per-type email opt-in. A missing entry falls back
// to the type's DefaultEmail.
type Preferences struct {
	TenantID    string
	UserID      string
	EmailByType map[Type]bool
	UpdatedAt   time.Time
}

// EmailEnabled reports the effective email setting for a type: the user's
// override if present, otherwise the type default.
func (p *Preferences) EmailEnabled(t Type) bool {
	if p != nil && p.EmailByType != nil {
		if v, ok := p.EmailByType[t]; ok {
			return v
		}
	}
	return DefaultEmail(t)
}

// Effective returns the full effective email map across all known types.
func (p *Preferences) Effective() map[Type]bool {
	out := make(map[Type]bool, len(AllTypes))
	for _, t := range AllTypes {
		out[t] = p.EmailEnabled(t)
	}
	return out
}

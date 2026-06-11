package entity

import "time"

// LegalHold marks a contact's data as under a legal obligation: it must not be
// anonymized (or deleted by retention) before the deadline. A zero Until means
// an indefinite hold. Holds are created out-of-band (e.g. by an administrator or
// a future legal-hold endpoint); the privacy domain only reads them to enforce
// "não anonimizar dados sob obrigação legal antes do prazo".
type LegalHold struct {
	ID        string
	TenantID  string
	ContactID string
	Reason    string
	Until     time.Time
	CreatedAt time.Time
}

// Active reports whether the hold is in force at the given time.
func (h *LegalHold) Active(at time.Time) bool {
	return h.Until.IsZero() || at.Before(h.Until)
}

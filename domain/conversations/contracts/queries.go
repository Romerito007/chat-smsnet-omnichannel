package contracts

// ListFilter narrows a conversation listing. Empty fields are ignored. Tenant
// scope and agent visibility are applied separately by the service.
type ListFilter struct {
	Status     string
	SectorID   string
	QueueID    string
	AssignedTo string
	ContactID  string
	Protocol   string
	Tag        string
}

// Visibility constrains which conversations an actor may see. When All is true
// the actor sees every conversation in the tenant; otherwise only those in one
// of SectorIDs or assigned to UserID.
type Visibility struct {
	All       bool
	SectorIDs []string
	UserID    string
}

// UnreadCounts is the number of conversations with unread messages
// (unread_count > 0) per inbox tab, for the badge on each tab. Each bucket uses
// the same filter its tab's list uses, so the badge always matches the list:
//   - Mine: assigned to the actor.
//   - Sector: in the actor's sectors (or every sector when the actor has all-scope).
//   - Queue: unassigned ("fila"), within the actor's visible sectors (or any when all-scope).
//
// Buckets are independent scopes and may overlap (a conversation assigned to me in
// my sector counts in both Mine and Sector), mirroring the per-tab lists.
type UnreadCounts struct {
	Mine   int
	Sector int
	Queue  int
}

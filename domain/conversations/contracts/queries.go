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

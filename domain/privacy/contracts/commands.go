package contracts

// UpdateRetention is the PATCH /v1/privacy/retention input. Pointer fields allow
// partial updates: a nil field leaves the current value unchanged.
type UpdateRetention struct {
	MessagesDays            *int
	ClosedConversationsDays *int
	TechnicalLogsDays       *int
	AuditLogsDays           *int
	NotificationsDays       *int
}

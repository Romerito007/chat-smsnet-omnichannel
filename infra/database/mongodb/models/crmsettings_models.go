package models

// CRMSettings is the BSON document for a tenant's CRM module toggles (one per
// tenant). The _id is the tenant id (singleton), so the upsert is naturally
// idempotent. timeline_enabled is always stored (no omitempty) so its false value is
// explicit.
type CRMSettings struct {
	Base            `bson:",inline"`
	TasksEnabled    bool `bson:"tasks_enabled"`
	ProductsEnabled bool `bson:"products_enabled"`
	TimelineEnabled bool `bson:"timeline_enabled"`
}

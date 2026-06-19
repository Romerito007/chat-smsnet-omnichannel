// Package entity holds the CRM settings aggregate: a per-tenant config that toggles
// the optional CRM modules (tasks, products, timeline) so each tenant enables only
// what it uses. One document per tenant.
package entity

import "time"

// Module identifies an optional CRM module that can be toggled per tenant.
type Module string

const (
	ModuleTasks    Module = "tasks"
	ModuleProducts Module = "products"
	ModuleTimeline Module = "timeline"
)

// CRMSettings is a tenant's CRM module configuration. Defaults are conservative:
// the timeline is on, tasks and products are off — a tenant turns them on when it
// adopts those modules, so the UI stays clean for everyone else.
type CRMSettings struct {
	TenantID        string
	TasksEnabled    bool
	ProductsEnabled bool
	TimelineEnabled bool
	UpdatedAt       time.Time
}

// Default returns the conservative defaults for a tenant that has never configured
// its CRM (timeline on, tasks/products off). UpdatedAt stays zero to signal "never
// configured".
func Default(tenantID string) *CRMSettings {
	return &CRMSettings{TenantID: tenantID, TimelineEnabled: true}
}

// Enabled reports whether the given module is on. Unknown modules are off.
func (s *CRMSettings) Enabled(module Module) bool {
	switch module {
	case ModuleTasks:
		return s.TasksEnabled
	case ModuleProducts:
		return s.ProductsEnabled
	case ModuleTimeline:
		return s.TimelineEnabled
	default:
		return false
	}
}

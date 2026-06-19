// Package contracts holds the CRM-settings service inputs.
package contracts

// UpdateCRMSettings carries the optional module toggles. A nil pointer leaves that
// flag unchanged (PATCH semantics).
type UpdateCRMSettings struct {
	TasksEnabled    *bool
	ProductsEnabled *bool
	TimelineEnabled *bool
}

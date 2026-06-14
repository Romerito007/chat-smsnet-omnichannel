// Package contracts holds the sector service inputs.
package contracts

// CreateSector is the input to create a sector. Enabled defaults to true.
type CreateSector struct {
	Name        string
	Description string
	Enabled     *bool
}

// UpdateSector carries optional fields; nil pointers mean "leave unchanged".
type UpdateSector struct {
	Name        *string
	Description *string
	Enabled     *bool
}

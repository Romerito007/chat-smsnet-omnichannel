package models

// Sector is the BSON document for a sector.
type Sector struct {
	Base          `bson:",inline"`
	Name          string         `bson:"name"`
	Description   string         `bson:"description"`
	Enabled       bool           `bson:"enabled"`
	BusinessHours map[string]any `bson:"business_hours,omitempty"`
}

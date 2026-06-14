package models

// CustomAttributeDefinition is the BSON document for a tenant custom-attribute
// definition. Unique on (tenant_id, applies_to, key).
type CustomAttributeDefinition struct {
	Base        `bson:",inline"`
	Key         string   `bson:"key"`
	Label       string   `bson:"label"`
	Description string   `bson:"description,omitempty"`
	Type        string   `bson:"type"`
	AppliesTo   string   `bson:"applies_to"`
	Options     []string `bson:"options,omitempty"`
	Regex       string   `bson:"regex,omitempty"`
}

package models

// Holiday is the BSON document for a tenant holiday. An empty SectorIDs with
// scope "all_sectors" applies to every sector.
type Holiday struct {
	Base      `bson:",inline"`
	Date      string   `bson:"date"`
	Name      string   `bson:"name"`
	Scope     string   `bson:"scope"`
	SectorIDs []string `bson:"sector_ids,omitempty"`
	Recurring bool     `bson:"recurring"`
}

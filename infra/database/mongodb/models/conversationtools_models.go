package models

// Tag is the BSON document for a conversation tag.
type Tag struct {
	Base        `bson:",inline"`
	Name        string `bson:"name"`
	Color       string `bson:"color,omitempty"`
	Description string `bson:"description,omitempty"`
	Enabled     bool   `bson:"enabled"`
}

// CannedResponse is the BSON document for a canned response. An empty SectorIDs
// means the response is global to the tenant.
type CannedResponse struct {
	Base      `bson:",inline"`
	SectorIDs []string `bson:"sector_ids,omitempty"`
	Shortcut  string   `bson:"shortcut"`
	Title     string   `bson:"title,omitempty"`
	Body      string   `bson:"body"`
	Enabled   bool     `bson:"enabled"`
}

// CloseReason is the BSON document for a close reason.
type CloseReason struct {
	Base         `bson:",inline"`
	Name         string `bson:"name"`
	RequiresNote bool   `bson:"requires_note"`
	Enabled      bool   `bson:"enabled"`
}

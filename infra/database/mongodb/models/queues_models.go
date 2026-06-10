package models

// Queue is the BSON document for a queue.
type Queue struct {
	Base           `bson:",inline"`
	SectorID       string `bson:"sector_id"`
	Name           string `bson:"name"`
	Strategy       string `bson:"strategy"`
	MaxWaitSeconds int    `bson:"max_wait_seconds"`
	Enabled        bool   `bson:"enabled"`
}

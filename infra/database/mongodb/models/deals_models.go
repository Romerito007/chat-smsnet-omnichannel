package models

import "time"

// Deal is the BSON document for a sales opportunity (the Kanban card).
type Deal struct {
	Base              `bson:",inline"`
	PipelineID        string     `bson:"pipeline_id"`
	StageID           string     `bson:"stage_id"`
	ContactID         string     `bson:"contact_id,omitempty"`
	Title             string     `bson:"title"`
	Value             float64    `bson:"value,omitempty"`
	Currency          string     `bson:"currency,omitempty"`
	AssignedTo        string     `bson:"assigned_to,omitempty"`
	SectorID          string     `bson:"sector_id,omitempty"`
	ConversationIDs   []string   `bson:"conversation_ids,omitempty"`
	Tags              []string   `bson:"tags,omitempty"`
	Source            string     `bson:"source,omitempty"`
	Status            string     `bson:"status"`
	LostReason        string     `bson:"lost_reason,omitempty"`
	ExpectedCloseDate *time.Time `bson:"expected_close_date,omitempty"`
	StageChangedAt    time.Time  `bson:"stage_changed_at"`
	ClosedAt          *time.Time `bson:"closed_at,omitempty"`
	Items             []DealItem `bson:"items,omitempty"`
}

// DealItem is the embedded BSON sub-document for a product line on a deal. Name and
// UnitPrice are a snapshot taken when the product was added.
type DealItem struct {
	ID        string  `bson:"id"`
	ProductID string  `bson:"product_id"`
	Name      string  `bson:"name"`
	Quantity  int     `bson:"quantity"`
	UnitPrice float64 `bson:"unit_price"`
	Total     float64 `bson:"total"`
}

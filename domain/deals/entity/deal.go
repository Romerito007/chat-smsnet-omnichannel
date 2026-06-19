// Package entity holds the Deal aggregate: a sales opportunity (the Kanban card)
// moving across a pipeline's stages.
package entity

import "time"

// Status is the deal lifecycle. A deal is open while in a non-terminal stage, and
// becomes won/lost when it enters an IsWon/IsLost stage.
type Status string

const (
	StatusOpen Status = "open"
	StatusWon  Status = "won"
	StatusLost Status = "lost"
)

// DefaultCurrency is used when a deal is created without one.
const DefaultCurrency = "BRL"

// Deal is a sales opportunity. It belongs to a pipeline and sits in one of its
// stages; ConversationIDs links the chat conversations it came from / relates to.
type Deal struct {
	ID         string
	TenantID   string
	PipelineID string
	StageID    string
	ContactID  string
	Title      string
	Value      float64
	Currency   string
	// AssignedTo is the seller (agent) handling the deal; optional.
	AssignedTo string
	// SectorID scopes visibility (mirrors conversations); optional.
	SectorID          string
	ConversationIDs   []string
	Source            string
	Status            Status
	LostReason        string
	ExpectedCloseDate *time.Time
	// StageChangedAt is bumped on every stage move (measures time-in-stage later).
	StageChangedAt time.Time
	ClosedAt       *time.Time
	// Items are the product line items (Products block). When present, Value is their
	// sum; with no items Value is edited manually (backward compatible).
	Items     []DealItem
	CreatedAt time.Time
	UpdatedAt time.Time
}

// DealItem is a product line on a deal. Name and UnitPrice are a SNAPSHOT taken when
// the product was added — a later catalog price change does not alter existing items.
type DealItem struct {
	ID        string
	ProductID string
	Name      string
	Quantity  int
	UnitPrice float64
	Total     float64
}

// RecalcValue recomputes each item's total and sets the deal Value to the sum of the
// items. It is called on every item mutation; with no items Value becomes 0 (and the
// deal is then editable manually again).
func (d *Deal) RecalcValue() {
	var sum float64
	for i := range d.Items {
		d.Items[i].Total = float64(d.Items[i].Quantity) * d.Items[i].UnitPrice
		sum += d.Items[i].Total
	}
	d.Value = sum
}

// FindItem returns the index of the item with the given id, or -1.
func (d *Deal) FindItem(itemID string) int {
	for i := range d.Items {
		if d.Items[i].ID == itemID {
			return i
		}
	}
	return -1
}

// HasConversation reports whether the deal already links the given conversation.
func (d *Deal) HasConversation(conversationID string) bool {
	for _, id := range d.ConversationIDs {
		if id == conversationID {
			return true
		}
	}
	return false
}

// Package entity holds the conversationtools entities: tags, canned responses
// and close reasons. All are tenant-scoped configuration used while handling
// conversations.
package entity

import "time"

// Tag is a tenant-defined label that can be applied to conversations.
type Tag struct {
	ID          string
	TenantID    string
	Name        string
	Color       string
	Description string
	Enabled     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// CannedResponse is a reusable reply template addressable by a shortcut. It is
// global to the tenant when SectorIDs is empty, or restricted to the listed
// sectors otherwise.
type CannedResponse struct {
	ID        string
	TenantID  string
	SectorIDs []string
	Shortcut  string
	Title     string
	Body      string
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Global reports whether the canned response is available to every sector.
func (c *CannedResponse) Global() bool { return len(c.SectorIDs) == 0 }

// VisibleToSectors reports whether the canned response is usable by an actor
// who belongs to any of the given sectors (always true when global).
func (c *CannedResponse) VisibleToSectors(sectorIDs []string) bool {
	if c.Global() {
		return true
	}
	for _, want := range c.SectorIDs {
		for _, have := range sectorIDs {
			if want == have {
				return true
			}
		}
	}
	return false
}

// CloseReason is a tenant-defined reason for closing a conversation. When
// RequiresNote is true, closing with this reason mandates a note.
type CloseReason struct {
	ID           string
	TenantID     string
	Name         string
	RequiresNote bool
	Enabled      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

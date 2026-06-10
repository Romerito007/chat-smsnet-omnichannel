package shared

import "github.com/google/uuid"

// ID is the canonical identifier type used by domain entities. We use string so
// the domain stays free of any storage-driver type (e.g. bson.ObjectID).
type ID = string

// NewID returns a new random identifier (UUIDv4).
func NewID() ID {
	return uuid.NewString()
}

// IsValidID reports whether s is a well-formed identifier.
func IsValidID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

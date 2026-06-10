package contracts

import "context"

// TagCatalog validates that tag ids belong to the tenant and are usable. It is
// implemented by the conversationtools domain and wired into the conversations
// service so applying tags can reject unknown/disabled tags. Optional: when
// unset, tag ids are accepted as-is.
type TagCatalog interface {
	ValidateTags(ctx context.Context, tagIDs []string) error
}

// CloseReasonPolicy reports whether a close reason requires a note. It is
// implemented by the conversationtools domain and wired into the conversations
// service so Close can enforce "requires_note". Optional: when unset, no note is
// required.
type CloseReasonPolicy interface {
	// RequiresNote returns whether the given close reason mandates a note. An
	// unknown reason should return a not_found error.
	RequiresNote(ctx context.Context, reasonID string) (bool, error)
}

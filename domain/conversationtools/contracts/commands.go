// Package contracts holds the conversationtools service inputs.
package contracts

// CreateTag is the input to create a tag.
type CreateTag struct {
	Name        string
	Color       string
	Description string
	Enabled     *bool
}

// UpdateTag patches a tag. Nil fields are left unchanged.
type UpdateTag struct {
	Name        *string
	Color       *string
	Description *string
	Enabled     *bool
}

// CreateCannedResponse is the input to create a canned response.
type CreateCannedResponse struct {
	SectorIDs []string
	Shortcut  string
	Title     string
	Body      string
	Enabled   *bool
}

// UpdateCannedResponse patches a canned response. Nil fields are left unchanged;
// a non-nil (possibly empty) SectorIDs replaces the restriction.
type UpdateCannedResponse struct {
	SectorIDs *[]string
	Shortcut  *string
	Title     *string
	Body      *string
	Enabled   *bool
}

// CreateCloseReason is the input to create a close reason.
type CreateCloseReason struct {
	Name         string
	RequiresNote *bool
	Enabled      *bool
}

// UpdateCloseReason patches a close reason. Nil fields are left unchanged.
type UpdateCloseReason struct {
	Name         *string
	RequiresNote *bool
	Enabled      *bool
}

// ApplyTags is the input to add/remove tags on a conversation.
type ApplyTags struct {
	Add    []string
	Remove []string
}

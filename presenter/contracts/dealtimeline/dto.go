// Package dealtimeline holds the request DTOs for the deal-timeline endpoints. The
// feed response is the service's contracts.FeedItem (already JSON-tagged).
package dealtimeline

import "strings"

// CommentRequest is the body of POST /v1/deals/{id}/timeline/comments.
type CommentRequest struct {
	Text string `json:"text"`
}

// TrimmedText returns the comment text trimmed of surrounding whitespace.
func (r CommentRequest) TrimmedText() string { return strings.TrimSpace(r.Text) }

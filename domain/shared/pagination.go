package shared

import (
	"encoding/base64"
	"encoding/json"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
)

// DefaultPageSize and MaxPageSize bound keyset pagination requests.
const (
	DefaultPageSize = 25
	MaxPageSize     = 100
)

// PageRequest describes a keyset (cursor) pagination request. Listings should
// use this instead of offset/limit to stay efficient on large collections.
type PageRequest struct {
	// Cursor is an opaque token returned by a previous page. Empty means the
	// first page.
	Cursor string
	// Limit is the maximum number of items to return.
	Limit int
}

// Normalize clamps the limit into the allowed range, applying the default when
// unset.
func (p PageRequest) Normalize() PageRequest {
	if p.Limit <= 0 {
		p.Limit = DefaultPageSize
	}
	if p.Limit > MaxPageSize {
		p.Limit = MaxPageSize
	}
	return p
}

// Cursor is the decoded keyset position. CreatedAt and ID together form a total
// order, which guarantees stable pagination even when many rows share the same
// timestamp.
type Cursor struct {
	CreatedAt int64 `json:"t"`
	ID        ID    `json:"i"`
}

// Encode serializes the cursor into an opaque, URL-safe token.
func (c Cursor) Encode() string {
	raw, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(raw)
}

// DecodeCursor parses an opaque token back into a Cursor. An empty token yields
// the zero Cursor and no error (meaning "start from the beginning").
func DecodeCursor(token string) (Cursor, error) {
	var c Cursor
	if token == "" {
		return c, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return c, apperror.Validation("invalid cursor").Wrap(err)
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return c, apperror.Validation("invalid cursor").Wrap(err)
	}
	return c, nil
}

// PageInfo is the pagination metadata returned alongside a page of data.
type PageInfo struct {
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}

// Page is the generic paginated response envelope:
//
//	{ "data": [...], "page": { "next_cursor": "...", "has_more": true } }
type Page[T any] struct {
	Data []T      `json:"data"`
	Page PageInfo `json:"page"`
}

// NewPage builds a Page from a slice of items that has been over-fetched by one
// element to detect whether more pages exist. `items` should contain up to
// limit+1 elements; nextCursor is derived from the last returned element via
// the provided cursor function.
func NewPage[T any](items []T, limit int, cursorOf func(T) Cursor) Page[T] {
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	info := PageInfo{HasMore: hasMore}
	if hasMore && len(items) > 0 {
		info.NextCursor = cursorOf(items[len(items)-1]).Encode()
	}
	return Page[T]{Data: items, Page: info}
}

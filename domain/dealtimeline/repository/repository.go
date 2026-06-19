// Package repository declares the deal-timeline persistence contract.
package repository

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/dealtimeline/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// TimelineRepository persists a deal's timeline events (tenant scope from ctx).
type TimelineRepository interface {
	// Append stores one event.
	Append(ctx context.Context, ev *entity.Event) error
	// ListByDeal returns a keyset page of a deal's events, most recent first. It
	// over-fetches by one (limit+1) so the caller can detect a next page.
	ListByDeal(ctx context.Context, dealID string, page shared.PageRequest) ([]*entity.Event, error)
}

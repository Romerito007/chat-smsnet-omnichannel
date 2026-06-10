// Package contracts holds the queue service inputs.
package contracts

import "github.com/romerito007/chat-smsnet-omnichannel/domain/queues/entity"

// CreateQueue is the input to create a queue. Strategy defaults to manual and
// Enabled to true.
type CreateQueue struct {
	SectorID       string
	Name           string
	Strategy       entity.Strategy
	MaxWaitSeconds int
	Enabled        *bool
}

// UpdateQueue carries optional fields; nil pointers mean "leave unchanged".
// SectorID is intentionally immutable after creation.
type UpdateQueue struct {
	Name           *string
	Strategy       *entity.Strategy
	MaxWaitSeconds *int
	Enabled        *bool
}

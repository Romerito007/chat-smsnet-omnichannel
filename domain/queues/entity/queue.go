// Package entity holds the Queue aggregate.
package entity

import "time"

// Strategy is the distribution strategy a queue uses to assign conversations.
type Strategy string

const (
	StrategyManual      Strategy = "manual"
	StrategyRoundRobin  Strategy = "round_robin"
	StrategyLeastLoaded Strategy = "least_loaded"
	StrategyPriority    Strategy = "priority"
)

// Valid reports whether s is a known strategy.
func (s Strategy) Valid() bool {
	switch s {
	case StrategyManual, StrategyRoundRobin, StrategyLeastLoaded, StrategyPriority:
		return true
	}
	return false
}

// Queue is a waiting line within a sector, with a distribution strategy and an
// optional max wait before escalation.
type Queue struct {
	ID             string
	TenantID       string
	SectorID       string
	Name           string
	Strategy       Strategy
	MaxWaitSeconds int
	Enabled        bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

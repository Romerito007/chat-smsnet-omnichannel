// Package entity holds the SLA domain entities: policies and per-conversation
// tracking.
package entity

import "time"

// SLAPolicy defines first-response and resolution targets for conversations that
// match its (optional) sector/priority/channel criteria. The most specific
// enabled policy wins.
type SLAPolicy struct {
	ID                     string
	TenantID               string
	Name                   string
	SectorIDs              []string
	Priority               string
	Channel                string
	FirstResponseTargetSec int
	ResolutionTargetSec    int
	BusinessHoursOnly      bool
	WarningThresholdPct    int
	// PauseOnWaitingCustomer pauses breach evaluation while the conversation is
	// waiting on the customer. Default false: the clock keeps running.
	PauseOnWaitingCustomer bool
	Enabled                bool
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

// Matches reports whether the policy applies to a conversation with the given
// sector, priority and channel. Empty criteria match anything.
func (p *SLAPolicy) Matches(sectorID, priority, channel string) bool {
	if len(p.SectorIDs) > 0 && !contains(p.SectorIDs, sectorID) {
		return false
	}
	if p.Priority != "" && p.Priority != priority {
		return false
	}
	if p.Channel != "" && p.Channel != channel {
		return false
	}
	return true
}

// Specificity scores how specific a policy is, so the best match can be picked
// (higher wins). Each set criterion adds weight.
func (p *SLAPolicy) Specificity() int {
	score := 0
	if len(p.SectorIDs) > 0 {
		score += 4
	}
	if p.Priority != "" {
		score += 2
	}
	if p.Channel != "" {
		score += 1
	}
	return score
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

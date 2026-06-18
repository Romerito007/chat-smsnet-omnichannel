// Package entity holds the sales-pipeline aggregate: a tenant-configurable Kanban
// funnel of stages. Deals (opportunities) move across these stages in a later block.
package entity

import (
	"sort"
	"strings"
	"time"
)

// Stage is one column of the Kanban funnel. Order is the column position. A stage
// may be terminal: IsWon (closed-won) or IsLost (closed-lost) — used later by the
// conversion metrics. Color is an optional UI hint.
type Stage struct {
	ID     string
	Name   string
	Order  int
	IsWon  bool
	IsLost bool
	Color  string
}

// Pipeline is a tenant's sales funnel. One pipeline per tenant is the default (used
// by the Kanban when no pipeline is selected).
type Pipeline struct {
	ID        string
	TenantID  string
	Name      string
	IsDefault bool
	Stages    []Stage
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SortStages orders the stages by Order ascending (stable), so the Kanban renders
// columns left-to-right.
func (p *Pipeline) SortStages() {
	sort.SliceStable(p.Stages, func(i, j int) bool { return p.Stages[i].Order < p.Stages[j].Order })
}

// StageIndex returns the index of the stage with the given id, or -1.
func (p *Pipeline) StageIndex(stageID string) int {
	for i := range p.Stages {
		if p.Stages[i].ID == stageID {
			return i
		}
	}
	return -1
}

// Validate enforces the pipeline invariants: a name, every stage named, no stage
// both won and lost, and AT MOST ONE won and ONE lost stage (so the conversion
// metrics have a single, unambiguous terminal of each kind). Returns "" when valid.
func (p *Pipeline) Validate() string {
	if strings.TrimSpace(p.Name) == "" {
		return "name is required"
	}
	won, lost := 0, 0
	for _, st := range p.Stages {
		if strings.TrimSpace(st.Name) == "" {
			return "every stage requires a name"
		}
		if st.IsWon && st.IsLost {
			return "a stage cannot be both won and lost"
		}
		if st.IsWon {
			won++
		}
		if st.IsLost {
			lost++
		}
	}
	if won > 1 {
		return "at most one won stage is allowed"
	}
	if lost > 1 {
		return "at most one lost stage is allowed"
	}
	return ""
}

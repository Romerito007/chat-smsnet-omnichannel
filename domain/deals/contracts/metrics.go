package contracts

import (
	"context"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// SalesFilter scopes the sales metrics: an optional pipeline and a period. The
// period bounds the won/lost (closed) figures by ClosedAt; open figures are a
// current snapshot. Empty From/To are open bounds.
type SalesFilter struct {
	PipelineID string
	From       time.Time
	To         time.Time
}

// CountValue is a {count, value-sum} aggregate keyed elsewhere.
type CountValue struct {
	Count int
	Value float64
}

// ── funnel ───────────────────────────────────────────────────────────────────

// FunnelStage is the open-deal aggregate for one stage (id resolved to a name at
// the service).
type FunnelStage struct {
	StageID   string  `json:"stage_id"`
	StageName string  `json:"stage_name,omitempty"`
	Count     int     `json:"count"`
	Value     float64 `json:"value"`
}

// SalesFunnel is the funnel view: open deals per stage plus the period totals.
type SalesFunnel struct {
	Stages         []FunnelStage `json:"stages"`
	OpenValue      float64       `json:"open_value"`
	WonCount       int           `json:"won_count"`
	WonValue       float64       `json:"won_value"`
	LostCount      int           `json:"lost_count"`
	LostValue      float64       `json:"lost_value"`
	ConversionRate float64       `json:"conversion_rate"`
}

// ── agents ───────────────────────────────────────────────────────────────────

// SalesAgent is the per-seller ranking row (assigned_to resolved to a name+avatar).
type SalesAgent struct {
	AgentID        string  `json:"agent_id"`
	AgentName      string  `json:"agent_name,omitempty"`
	AgentAvatarURL string  `json:"agent_avatar_url,omitempty"`
	WonCount       int     `json:"won_count"`
	WonValue       float64 `json:"won_value"`
	LostCount      int     `json:"lost_count"`
	LostValue      float64 `json:"lost_value"`
	OpenValue      float64 `json:"open_value"`
	ConversionRate float64 `json:"conversion_rate"`
}

// SalesAgents is the seller ranking, ordered by won value desc.
type SalesAgents struct {
	Agents []SalesAgent `json:"agents"`
}

// ── cycle ────────────────────────────────────────────────────────────────────

// StageDwell is the average current dwell time (seconds) of OPEN deals in a stage —
// an approximation of time-in-stage (only the last transition is stored).
type StageDwell struct {
	StageID    string  `json:"stage_id"`
	StageName  string  `json:"stage_name,omitempty"`
	OpenCount  int     `json:"open_count"`
	AvgSeconds float64 `json:"avg_seconds"`
}

// StalledDeal is an open deal that has not changed stage for a while.
type StalledDeal struct {
	ID             string  `json:"id"`
	Title          string  `json:"title"`
	StageID        string  `json:"stage_id"`
	StageName      string  `json:"stage_name,omitempty"`
	AssignedTo     string  `json:"assigned_to,omitempty"`
	AssignedToName string  `json:"assigned_to_name,omitempty"`
	Value          float64 `json:"value"`
	DaysInStage    int     `json:"days_in_stage"`
}

// SalesCycle is the cycle-time view.
type SalesCycle struct {
	AvgCloseSeconds float64       `json:"avg_close_seconds"`
	WonCount        int           `json:"won_count"`
	ByStage         []StageDwell  `json:"by_stage"`
	Stalled         []StalledDeal `json:"stalled"`
}

// AgentDirectory resolves agent ids to display cards (name + avatar) so the sales
// metrics render the seller, not a raw id. Satisfied by the IAM user service. Optional.
type AgentDirectory interface {
	AgentCards(ctx context.Context, userIDs []string) (map[string]shared.DisplayCard, error)
}

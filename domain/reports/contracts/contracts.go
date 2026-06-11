// Package contracts holds the reports query filter, the report result types and
// the ReportService abstraction. The MVP backs it with Mongo aggregations; a
// dedicated engine (ClickHouse) can implement the same interface later, and the
// reports.snapshot job pre-aggregates into reports_snapshots for future use.
package contracts

import (
	"context"
	"time"
)

// Filter scopes every report. From/To bound the period; the others narrow by
// sector, agent and channel. Empty string filters are ignored.
type Filter struct {
	From       time.Time
	To         time.Time
	SectorID   string
	AssignedTo string
	Channel    string
}

// Bucket is a generic key→count aggregation row.
type Bucket struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

// DateCount is a per-day count.
type DateCount struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// AgentStat is per-agent productivity.
type AgentStat struct {
	AgentID              string  `json:"agent_id"`
	Conversations        int     `json:"conversations"`
	AvgResolutionSeconds float64 `json:"avg_resolution_seconds"`
}

// SectorStat is per-sector volume.
type SectorStat struct {
	SectorID      string `json:"sector_id"`
	Conversations int    `json:"conversations"`
}

// Overview is the headline summary.
type Overview struct {
	From                       time.Time `json:"from"`
	To                         time.Time `json:"to"`
	TotalConversations         int       `json:"total_conversations"`
	OpenByStatus               []Bucket  `json:"open_by_status"`
	Messages                   int       `json:"messages"`
	FirstResponseAvgSec        float64   `json:"first_response_avg_seconds"`
	ResolutionAvgSec           float64   `json:"resolution_avg_seconds"`
	CSATAvgScore               float64   `json:"csat_avg_score"`
	CSATResponseRate           float64   `json:"csat_response_rate"`
	SLAFirstResponseBreachRate float64   `json:"sla_first_response_breach_rate"`
	SLAResolutionBreachRate    float64   `json:"sla_resolution_breach_rate"`
}

// ConversationsReport breaks conversations down several ways.
type ConversationsReport struct {
	Daily          []DateCount `json:"daily"`
	ByStatus       []Bucket    `json:"by_status"`
	BySector       []Bucket    `json:"by_sector"`
	ByChannel      []Bucket    `json:"messages_by_channel"`
	ClosedByReason []Bucket    `json:"closed_by_reason"`
}

// AgentsReport is the per-agent breakdown.
type AgentsReport struct {
	Agents []AgentStat `json:"agents"`
}

// SectorsReport is the per-sector breakdown.
type SectorsReport struct {
	Sectors []SectorStat `json:"sectors"`
}

// AutomationReport summarizes automation runs.
type AutomationReport struct {
	Total    int      `json:"total"`
	ByStatus []Bucket `json:"by_status"`
}

// CopilotReport summarizes AI copilot usage.
type CopilotReport struct {
	TotalCalls    int      `json:"total_calls"`
	ByAction      []Bucket `json:"by_action"`
	TokensInput   int      `json:"tokens_input"`
	TokensOutput  int      `json:"tokens_output"`
	EstimatedCost float64  `json:"estimated_cost"`
}

// SLAReport summarizes SLA tracking outcomes.
type SLAReport struct {
	Tracked                 int     `json:"tracked"`
	FirstResponseBreached   int     `json:"first_response_breached"`
	ResolutionBreached      int     `json:"resolution_breached"`
	Met                     int     `json:"met"`
	FirstResponseBreachRate float64 `json:"first_response_breach_rate"`
	ResolutionBreachRate    float64 `json:"resolution_breach_rate"`
}

// CSATReport summarizes satisfaction surveys.
type CSATReport struct {
	Sent         int      `json:"sent"`
	Responded    int      `json:"responded"`
	Expired      int      `json:"expired"`
	AvgScore     float64  `json:"avg_score"`
	ResponseRate float64  `json:"response_rate"`
	ByScore      []Bucket `json:"by_score"`
}

// ReportService is the reporting abstraction. Every method enforces tenant +
// report.view (the latter at the HTTP layer).
type ReportService interface {
	Overview(ctx context.Context, f Filter) (Overview, error)
	Conversations(ctx context.Context, f Filter) (ConversationsReport, error)
	Agents(ctx context.Context, f Filter) (AgentsReport, error)
	Sectors(ctx context.Context, f Filter) (SectorsReport, error)
	Automation(ctx context.Context, f Filter) (AutomationReport, error)
	Copilot(ctx context.Context, f Filter) (CopilotReport, error)
	SLA(ctx context.Context, f Filter) (SLAReport, error)
	CSAT(ctx context.Context, f Filter) (CSATReport, error)
	// RequestExport audits + enqueues a report export (report.export permission).
	RequestExport(ctx context.Context, report, format string, f Filter) error
}

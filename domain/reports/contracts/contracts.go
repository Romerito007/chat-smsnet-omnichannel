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

// Bucket is a generic key→count aggregation row. Label is the resolved display
// name when the key is a raw id (sector, close reason), filled in batch at the
// presenter so the dashboard renders the name instead of the id; empty when the key
// is already human-readable (status, channel, score) or unresolved.
type Bucket struct {
	Key   string `json:"key"`
	Label string `json:"label,omitempty"`
	Count int    `json:"count"`
}

// DateCount is a per-day count.
type DateCount struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// AgentStat is per-agent productivity. Name is the resolved display name (filled
// at the presenter so the dashboard never shows a raw id); empty when unknown.
type AgentStat struct {
	AgentID              string  `json:"agent_id"`
	Name                 string  `json:"name,omitempty"`
	AvatarURL            string  `json:"avatar_url,omitempty"`
	Conversations        int     `json:"conversations"`
	AvgResolutionSeconds float64 `json:"avg_resolution_seconds"`
}

// SectorStat is per-sector volume. Name is the resolved sector name (filled at the
// presenter); empty when unknown or for sector-less conversations.
type SectorStat struct {
	SectorID      string `json:"sector_id"`
	Name          string `json:"name,omitempty"`
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

// CopilotReport summarizes AI copilot usage.
type CopilotReport struct {
	TotalCalls    int      `json:"total_calls"`
	ByAction      []Bucket `json:"by_action"`
	TokensInput   int      `json:"tokens_input"`
	TokensOutput  int      `json:"tokens_output"`
	EstimatedCost float64  `json:"estimated_cost"`
}

// AutomationReport summarizes automation-rule firings (one row per executed action
// in rule_evaluation_logs) over the period.
type AutomationReport struct {
	TotalEvaluations int      `json:"total_evaluations"`
	ByStatus         []Bucket `json:"by_status"` // applied | skipped_missing_ref | failed | …
	ByEvent          []Bucket `json:"by_event"`  // conversation.created | message.created | …
	ByAction         []Bucket `json:"by_action"` // send_message | assign_agent | …
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

// ExportResult is the outcome of a report export: a real file written to the
// store and a temporary signed URL to download it.
type ExportResult struct {
	Report      string    `json:"report"`
	Format      string    `json:"format"`
	Filename    string    `json:"filename"`
	DownloadURL string    `json:"download_url"`
	ExpiresAt   time.Time `json:"expires_at"`
	Bytes       int       `json:"bytes"`
}

// FileStore persists export artifacts and mints temporary, signed download URLs.
// Implemented by infra/storage (reused across domains).
type FileStore interface {
	Save(key string, data []byte) error
	SignedURL(key string, ttl time.Duration) (url string, expiresAt time.Time, err error)
	Resolve(token string) (key string, err error)
	Open(key string) (data []byte, filename string, err error)
}

// ReportService is the reporting abstraction. Every method enforces tenant +
// report.view (the latter at the HTTP layer).
type ReportService interface {
	Overview(ctx context.Context, f Filter) (Overview, error)
	Conversations(ctx context.Context, f Filter) (ConversationsReport, error)
	Agents(ctx context.Context, f Filter) (AgentsReport, error)
	Sectors(ctx context.Context, f Filter) (SectorsReport, error)
	Copilot(ctx context.Context, f Filter) (CopilotReport, error)
	Automation(ctx context.Context, f Filter) (AutomationReport, error)
	SLA(ctx context.Context, f Filter) (SLAReport, error)
	CSAT(ctx context.Context, f Filter) (CSATReport, error)
	// Export renders the named report (json|csv), stores the file and returns a
	// signed download URL (report.export permission).
	Export(ctx context.Context, report, format string, f Filter) (ExportResult, error)
}

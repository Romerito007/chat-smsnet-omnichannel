// Package repository declares the reports aggregation port. The Mongo
// implementation runs aggregation pipelines; a dedicated engine can implement
// the same interface later.
package repository

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/reports/contracts"
)

// CSATRaw is the raw CSAT aggregation; the service derives the average and rate.
type CSATRaw struct {
	Sent       int
	Responded  int
	Expired    int
	ScoreSum   int
	ScoreCount int
	ByScore    []contracts.Bucket
}

// Repository is the reports aggregation backend. All reads are tenant-scoped from
// the context; every method honors the filter (period + sector/agent/channel).
type Repository interface {
	CountConversations(ctx context.Context, f contracts.Filter) (int, error)
	CountMessages(ctx context.Context, f contracts.Filter) (int, error)
	OpenByStatus(ctx context.Context, f contracts.Filter) ([]contracts.Bucket, error)
	ConversationsByStatus(ctx context.Context, f contracts.Filter) ([]contracts.Bucket, error)
	ConversationsDaily(ctx context.Context, f contracts.Filter) ([]contracts.DateCount, error)
	ConversationsBySector(ctx context.Context, f contracts.Filter) ([]contracts.Bucket, error)
	ClosedByReason(ctx context.Context, f contracts.Filter) ([]contracts.Bucket, error)
	MessagesByChannel(ctx context.Context, f contracts.Filter) ([]contracts.Bucket, error)
	FirstResponseAvgSeconds(ctx context.Context, f contracts.Filter) (float64, error)
	ResolutionAvgSeconds(ctx context.Context, f contracts.Filter) (float64, error)
	AgentStats(ctx context.Context, f contracts.Filter) ([]contracts.AgentStat, error)
	SectorStats(ctx context.Context, f contracts.Filter) ([]contracts.SectorStat, error)
	CopilotUsage(ctx context.Context, f contracts.Filter) (contracts.CopilotReport, error)
	// SLACounts returns the raw tracking counts (rates derived by the service).
	SLACounts(ctx context.Context, f contracts.Filter) (contracts.SLAReport, error)
	// CSAT returns the raw survey aggregation (average/rate derived by the service).
	CSAT(ctx context.Context, f contracts.Filter) (CSATRaw, error)
}

// Package contracts holds the deal-timeline service inputs, the ports it depends on
// (deal lookup for visibility, module gate for the toggle, name directories) and the
// enriched feed output.
package contracts

import (
	"context"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/dealtimeline/entity"
	pipelineentity "github.com/romerito007/chat-smsnet-omnichannel/domain/pipelines/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// RecordEvent is the write input used by the automatic instrumentation (the deals
// service today; tasks/products later) to append a timeline event.
type RecordEvent struct {
	DealID  string
	Kind    entity.Kind
	ActorID string
	Data    map[string]any
}

// DealRef is the minimal deal data the timeline needs to apply the deal's visibility
// (the same rule as the deal list) and to resolve stage names.
type DealRef struct {
	TenantID   string
	SectorID   string
	AssignedTo string
	PipelineID string
}

// DealLookup resolves a deal (tenant-scoped) for the visibility check + stage names.
// Implemented over the deals repository.
type DealLookup interface {
	Deal(ctx context.Context, dealID string) (*DealRef, error)
}

// ModuleGate reports whether the timeline module is enabled for the tenant.
// Implemented over the crmsettings service (IsModuleEnabled). Optional: a nil gate
// means always enabled.
type ModuleGate interface {
	TimelineEnabled(ctx context.Context) (bool, error)
}

// AgentDirectory resolves user ids to display cards (name + avatar) so the feed shows
// the actor (and the from/to seller on an assignment change), not a raw id. Optional.
type AgentDirectory interface {
	AgentCards(ctx context.Context, userIDs []string) (map[string]shared.DisplayCard, error)
}

// PipelineLookup resolves a pipeline (with its stages) so the feed shows stage names.
// Implemented over the pipelines service. Optional.
type PipelineLookup interface {
	Get(ctx context.Context, pipelineID string) (*pipelineentity.Pipeline, error)
}

// FeedItem is one enriched timeline entry: the raw event plus the resolved actor
// name/avatar; Data already carries the resolved stage/seller names (never raw ids).
type FeedItem struct {
	ID             string         `json:"id"`
	DealID         string         `json:"deal_id"`
	Kind           string         `json:"kind"`
	ActorID        string         `json:"actor_id"`
	ActorName      string         `json:"actor_name,omitempty"`
	ActorAvatarURL string         `json:"actor_avatar_url,omitempty"`
	Data           map[string]any `json:"data,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
}

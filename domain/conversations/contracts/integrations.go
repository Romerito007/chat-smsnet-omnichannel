package contracts

import (
	"context"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
)

// TagCatalog validates and resolves tags against the tenant's catalog. It is
// implemented by the conversationtools domain and wired into the conversations
// service so applying tags can reject unknown/disabled tags and store canonical
// ids. Optional: when unset, tags are accepted as-is.
type TagCatalog interface {
	ValidateTags(ctx context.Context, tagIDs []string) error
	// ResolveTags maps each ref (a tag id OR a tag name) to its canonical id, so a
	// conversation's tags array is ALWAYS ids. strict=true returns a validation
	// error for an unknown/disabled ref (used for add); strict=false passes an
	// unresolved ref through unchanged (used for remove, so a stale value can still
	// be stripped). The result is de-duplicated.
	ResolveTags(ctx context.Context, refs []string, strict bool) ([]string, error)
}

// CloseReasonPolicy reports whether a close reason requires a note. It is
// implemented by the conversationtools domain and wired into the conversations
// service so Close can enforce "requires_note". Optional: when unset, no note is
// required.
type CloseReasonPolicy interface {
	// RequiresNote returns whether the given close reason mandates a note. An
	// unknown reason should return a not_found error.
	RequiresNote(ctx context.Context, reasonID string) (bool, error)
}

// SLAHook lets the SLA domain observe a conversation's lifecycle so it can
// create/advance SLA tracking. It is implemented by the sla domain and wired
// into the conversations service. Every method is best-effort and side-effect
// only: an SLA failure must never break the conversation operation.
type SLAHook interface {
	// OnConversationCreated selects an applicable policy and starts tracking.
	OnConversationCreated(ctx context.Context, conv *entity.Conversation)
	// OnFirstResponse records the first agent response time (idempotent: only the
	// first call per conversation has effect).
	OnFirstResponse(ctx context.Context, conv *entity.Conversation, at time.Time)
	// OnResolved records the resolution time and finalizes the tracking.
	OnResolved(ctx context.Context, conv *entity.Conversation, at time.Time)
}

// NoopSLAHook ignores every lifecycle event. The default when no SLA domain is
// wired.
type NoopSLAHook struct{}

func (NoopSLAHook) OnConversationCreated(context.Context, *entity.Conversation)      {}
func (NoopSLAHook) OnFirstResponse(context.Context, *entity.Conversation, time.Time) {}
func (NoopSLAHook) OnResolved(context.Context, *entity.Conversation, time.Time)      {}

// CSATTrigger is notified when a conversation is closed, so the CSAT domain can
// enqueue a satisfaction survey when the conversation is eligible. Best-effort:
// a CSAT failure must never break the close. Implemented by the csat domain.
type CSATTrigger interface {
	OnConversationClosed(ctx context.Context, conv *entity.Conversation)
}

// NoopCSATTrigger ignores closes. The default when no CSAT domain is wired.
type NoopCSATTrigger struct{}

func (NoopCSATTrigger) OnConversationClosed(context.Context, *entity.Conversation) {}

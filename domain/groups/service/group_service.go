// Package service holds the WhatsApp groups business logic: the gateway sync
// upsert, the management listing/filter and the sync request to the gateway.
package service

import (
	"context"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/groups/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/groups/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/groups/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// maxSyncBatch bounds one upsert batch (the gateway splits ~5k groups into batches).
const maxSyncBatch = 2000

// eventGroupSyncRequested is the event the chat sends to the channel's managed
// webhook to ask the gateway to push the group list. It is NOT in the public
// webhook catalog (SupportedEvents) — it is a request to one specific gateway.
const eventGroupSyncRequested = "group_sync_requested"

// Service manages known WhatsApp groups.
type Service struct {
	repo    repository.GroupRepository
	emitter contracts.ChannelEventEmitter
	auditor shared.Auditor
	clock   shared.Clock
}

// New builds the service. The emitter (channel webhook) is optional: without it,
// Sync returns an error.
func New(repo repository.GroupRepository, emitter contracts.ChannelEventEmitter, clock shared.Clock) *Service {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &Service{repo: repo, emitter: emitter, auditor: shared.NoopAuditor{}, clock: clock}
}

// SetAuditor wires the audit trail (optional).
func (s *Service) SetAuditor(a shared.Auditor) {
	if a != nil {
		s.auditor = a
	}
}

// UpsertBatch idempotently stores a gateway sync batch. Entries with no group_jid
// are skipped (defensive). Tenant comes from the context (the channel inbound token).
func (s *Service) UpsertBatch(ctx context.Context, channelID string, groups []contracts.UpsertGroup) (int, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return 0, err
	}
	if len(groups) == 0 {
		return 0, apperror.Validation("groups is required")
	}
	if len(groups) > maxSyncBatch {
		return 0, apperror.Validation("batch too large").
			WithDetails(map[string]any{"groups": "at most 2000 per batch; split the sync"})
	}
	clean := make([]contracts.UpsertGroup, 0, len(groups))
	for _, g := range groups {
		g.GroupJID = strings.TrimSpace(g.GroupJID)
		if g.GroupJID == "" {
			continue // a group with no JID cannot be keyed — skip
		}
		clean = append(clean, g)
	}
	if len(clean) == 0 {
		return 0, apperror.Validation("no group with a group_jid in the batch")
	}
	return s.repo.UpsertBatch(ctx, strings.TrimSpace(channelID), clean)
}

// List returns the tenant's groups for the management screen.
func (s *Service) List(ctx context.Context, f contracts.ListFilter, page shared.PageRequest) ([]*entity.Group, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, f, page.Normalize())
}

// SetAttend toggles the attendance filter on a group (the management screen).
func (s *Service) SetAttend(ctx context.Context, id string, attend bool) (*entity.Group, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	g, err := s.repo.SetAttend(ctx, id, attend)
	if err != nil {
		return nil, err
	}
	_ = s.auditor.Record(ctx, shared.AuditEntry{
		Action: "group.attend_changed", ResourceType: "group", ResourceID: g.ID,
		Data: map[string]any{"attend": attend},
	})
	return g, nil
}

// FindByJID returns a group by JID (tenant-scoped). Exposed for Domain 2's attend
// gate (before creating a conversation for a group message).
func (s *Service) FindByJID(ctx context.Context, groupJID string) (*entity.Group, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.FindByJID(ctx, strings.TrimSpace(groupJID))
}

// Sync asks the gateway to push the channel's group list, by emitting
// group_sync_requested to the channel's MANAGED webhook. The gateway returns the
// groups in batches at POST /v1/inbound/channel/{channel}/groups. Returns an error
// when the channel has no managed webhook (no outbound_url) — the sync cannot work.
func (s *Service) Sync(ctx context.Context, channelID string) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return apperror.Validation("channel_id is required")
	}
	if s.emitter == nil {
		return apperror.Integration("group sync is not configured")
	}
	if err := s.emitter.EmitToChannel(ctx, tenantID, channelID, eventGroupSyncRequested, map[string]any{"channel_id": channelID}); err != nil {
		return err
	}
	_ = s.auditor.Record(ctx, shared.AuditEntry{
		Action: "group.sync_requested", ResourceType: "channel", ResourceID: channelID,
	})
	return nil
}

// Package service holds the deal-timeline business logic: append automatic/manual
// events, and read the chronological feed (most recent first) — respecting the
// tenant's timeline toggle, the deal's visibility, and resolving ids to names in
// batch (never a raw id).
package service

import (
	"context"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/dealtimeline/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/dealtimeline/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/dealtimeline/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// Service manages a deal's timeline.
type Service struct {
	repo      repository.TimelineRepository
	deals     contracts.DealLookup
	gate      contracts.ModuleGate
	agents    contracts.AgentDirectory
	pipelines contracts.PipelineLookup
	clock     shared.Clock
}

// New builds the service. The deal lookup is required for the visibility checks on
// reads/comments; the gate and directories are optional.
func New(repo repository.TimelineRepository, deals contracts.DealLookup, clock shared.Clock) *Service {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &Service{repo: repo, deals: deals, clock: clock}
}

// SetModuleGate wires the timeline on/off toggle (crmsettings). Optional.
func (s *Service) SetModuleGate(g contracts.ModuleGate) {
	if g != nil {
		s.gate = g
	}
}

// SetDirectories wires the actor/seller (agent) and stage (pipeline) name resolvers.
// Optional: without them the feed carries raw ids.
func (s *Service) SetDirectories(agents contracts.AgentDirectory, pipelines contracts.PipelineLookup) *Service {
	s.agents = agents
	s.pipelines = pipelines
	return s
}

// Record appends an automatic event. Best-effort and side-effect-free for the
// caller: it never returns an error (a timeline failure must not break the deal
// action that produced it) and ALWAYS writes — even when the module is toggled off,
// so the history stays complete; only reads hide it.
func (s *Service) Record(ctx context.Context, in contracts.RecordEvent) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil || strings.TrimSpace(in.DealID) == "" || !entity.ValidKind(in.Kind) {
		return
	}
	_ = s.repo.Append(ctx, &entity.Event{
		ID:        shared.NewID(),
		TenantID:  tenantID,
		DealID:    in.DealID,
		Kind:      in.Kind,
		ActorID:   in.ActorID,
		Data:      in.Data,
		CreatedAt: s.clock.Now(),
	})
}

// Comment appends a manual seller comment (kind=comment) authored by the actor. It
// requires the timeline to be enabled and the deal to be visible to the actor.
func (s *Service) Comment(ctx context.Context, dealID, text string) (*contracts.FeedItem, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, apperror.Validation("text is required")
	}
	on, err := s.enabled(ctx)
	if err != nil {
		return nil, err
	}
	if !on {
		return nil, apperror.Conflict("the timeline module is disabled for this tenant")
	}
	ref, err := s.ensureVisible(ctx, dealID)
	if err != nil {
		return nil, err
	}
	ev := &entity.Event{
		ID:        shared.NewID(),
		TenantID:  tenantID,
		DealID:    dealID,
		Kind:      entity.KindComment,
		ActorID:   actorFromCtx(ctx),
		Data:      map[string]any{"text": text},
		CreatedAt: s.clock.Now(),
	}
	if err := s.repo.Append(ctx, ev); err != nil {
		return nil, err
	}
	items := s.enrich(ctx, ref, []*entity.Event{ev})
	return &items[0], nil
}

// Feed returns a deal's timeline, most recent first (keyset-paginated). When the
// module is disabled for the tenant it returns an empty feed (the UI hides the tab);
// otherwise it enforces the deal's visibility and enriches ids to names.
func (s *Service) Feed(ctx context.Context, dealID string, page shared.PageRequest) ([]contracts.FeedItem, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	on, err := s.enabled(ctx)
	if err != nil {
		return nil, err
	}
	if !on {
		return []contracts.FeedItem{}, nil
	}
	ref, err := s.ensureVisible(ctx, dealID)
	if err != nil {
		return nil, err
	}
	events, err := s.repo.ListByDeal(ctx, dealID, page.Normalize())
	if err != nil {
		return nil, err
	}
	return s.enrich(ctx, ref, events), nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

// enabled reports whether the timeline module is on (default on when no gate).
func (s *Service) enabled(ctx context.Context) (bool, error) {
	if s.gate == nil {
		return true, nil
	}
	return s.gate.TimelineEnabled(ctx)
}

// ensureVisible loads the deal (tenant-scoped) and enforces the deal visibility: an
// all-scope actor sees all; otherwise only its assignee or a member of its sector.
// A non-visible deal is reported as NotFound (it is hidden, not 403'd).
func (s *Service) ensureVisible(ctx context.Context, dealID string) (*contracts.DealRef, error) {
	ref, err := s.deals.Deal(ctx, dealID)
	if err != nil {
		return nil, err
	}
	ac, ok := authz.FromContext(ctx)
	if !ok {
		return nil, apperror.Unauthorized("authentication required")
	}
	if ac.SectorScope == authz.ScopeAll || ref.AssignedTo == ac.UserID {
		return ref, nil
	}
	for _, sid := range ac.SectorIDs {
		if sid != "" && sid == ref.SectorID {
			return ref, nil
		}
	}
	return nil, apperror.NotFound("deal not found")
}

// enrich resolves the actor name/avatar (and the from/to seller on an assignment
// change) in ONE agent-directory call, and the stage names from the deal's pipeline
// in ONE lookup. Best-effort: a nil directory or a lookup error leaves raw ids.
func (s *Service) enrich(ctx context.Context, ref *contracts.DealRef, events []*entity.Event) []contracts.FeedItem {
	cards := s.agentCards(ctx, events)
	stages := s.stageNames(ctx, ref)

	items := make([]contracts.FeedItem, 0, len(events))
	for _, ev := range events {
		it := contracts.FeedItem{
			ID: ev.ID, DealID: ev.DealID, Kind: string(ev.Kind), ActorID: ev.ActorID,
			Data: copyData(ev.Data), CreatedAt: ev.CreatedAt,
		}
		if card, ok := cards[ev.ActorID]; ok {
			it.ActorName = card.Name
			it.ActorAvatarURL = card.AvatarURL
		}
		labelData(it.Data, ev.Kind, cards, stages)
		items = append(items, it)
	}
	return items
}

// agentCards batches every user id referenced by the events (actors + the from/to of
// an assignment change) into one directory call.
func (s *Service) agentCards(ctx context.Context, events []*entity.Event) map[string]shared.DisplayCard {
	if s.agents == nil {
		return map[string]shared.DisplayCard{}
	}
	set := map[string]struct{}{}
	add := func(id string) {
		if id != "" && id != entity.ActorSystem && id != entity.ActorAutomation {
			set[id] = struct{}{}
		}
	}
	for _, ev := range events {
		add(ev.ActorID)
		if ev.Kind == entity.KindAssignedChanged {
			add(dataString(ev.Data, "from"))
			add(dataString(ev.Data, "to"))
		}
	}
	if len(set) == 0 {
		return map[string]shared.DisplayCard{}
	}
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	cards, err := s.agents.AgentCards(ctx, ids)
	if err != nil {
		return map[string]shared.DisplayCard{}
	}
	return cards
}

// stageNames loads the deal's pipeline once and maps stage id → name.
func (s *Service) stageNames(ctx context.Context, ref *contracts.DealRef) map[string]string {
	out := map[string]string{}
	if s.pipelines == nil || ref == nil || ref.PipelineID == "" {
		return out
	}
	pl, err := s.pipelines.Get(ctx, ref.PipelineID)
	if err != nil {
		return out
	}
	for _, st := range pl.Stages {
		out[st.ID] = st.Name
	}
	return out
}

// labelData injects resolved names into a copy of the event data, by kind.
func labelData(data map[string]any, kind entity.Kind, cards map[string]shared.DisplayCard, stages map[string]string) {
	if data == nil {
		return
	}
	switch kind {
	case entity.KindStageChanged:
		setName(data, "from_stage_id", "from_stage_name", stages)
		setName(data, "to_stage_id", "to_stage_name", stages)
	case entity.KindDealCreated, entity.KindWon, entity.KindLost:
		setName(data, "stage_id", "stage_name", stages)
	case entity.KindAssignedChanged:
		setCard(data, "from", "from_name", cards)
		setCard(data, "to", "to_name", cards)
	}
}

func setName(data map[string]any, idKey, nameKey string, names map[string]string) {
	if id := dataString(data, idKey); id != "" {
		if n, ok := names[id]; ok {
			data[nameKey] = n
		}
	}
}

func setCard(data map[string]any, idKey, nameKey string, cards map[string]shared.DisplayCard) {
	if id := dataString(data, idKey); id != "" {
		if c, ok := cards[id]; ok {
			data[nameKey] = c.Name
		}
	}
}

func dataString(data map[string]any, key string) string {
	if data == nil {
		return ""
	}
	s, _ := data[key].(string)
	return s
}

func copyData(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in)+2)
	for k, v := range in {
		out[k] = v
	}
	return out
}

func actorFromCtx(ctx context.Context) string {
	if ac, ok := authz.FromContext(ctx); ok && ac.UserID != "" {
		return ac.UserID
	}
	return entity.ActorSystem
}

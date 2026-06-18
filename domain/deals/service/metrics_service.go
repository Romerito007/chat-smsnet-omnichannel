package service

import (
	"context"
	"sort"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/deals/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/deals/repository"
	pipelineentity "github.com/romerito007/chat-smsnet-omnichannel/domain/pipelines/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// defaultStalledDays is the dwell threshold for "stalled" open deals when the caller
// passes none.
const defaultStalledDays = 14

// SalesMetrics computes the sales-funnel reports over the deals collection,
// tenant-scoped and constrained by the actor's visibility (same as the deal list).
// Names (stage, agent) are resolved in batch — never raw ids.
type SalesMetrics struct {
	repo      repository.DealRepository
	pipelines contracts.PipelineLookup
	agents    contracts.AgentDirectory
	clock     shared.Clock
}

// NewSalesMetrics builds the metrics service. The agent directory is optional (a nil
// one leaves agent rows id-only).
func NewSalesMetrics(repo repository.DealRepository, pipelines contracts.PipelineLookup, clock shared.Clock) *SalesMetrics {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &SalesMetrics{repo: repo, pipelines: pipelines, clock: clock}
}

// SetAgentDirectory wires the seller name+avatar resolver.
func (s *SalesMetrics) SetAgentDirectory(a contracts.AgentDirectory) {
	if a != nil {
		s.agents = a
	}
}

// Funnel returns the funnel view: open deals per stage + the period won/lost totals
// and the conversion rate. Stage ids are resolved to names.
func (s *SalesMetrics) Funnel(ctx context.Context, f contracts.SalesFilter) (contracts.SalesFunnel, error) {
	vis, err := s.visibility(ctx)
	if err != nil {
		return contracts.SalesFunnel{}, err
	}
	stages, err := s.repo.OpenByStage(ctx, f, vis)
	if err != nil {
		return contracts.SalesFunnel{}, err
	}
	won, err := s.repo.ClosedTotals(ctx, "won", f, vis)
	if err != nil {
		return contracts.SalesFunnel{}, err
	}
	lost, err := s.repo.ClosedTotals(ctx, "lost", f, vis)
	if err != nil {
		return contracts.SalesFunnel{}, err
	}

	s.labelStages(ctx, f.PipelineID, stages)
	openValue := 0.0
	for _, st := range stages {
		openValue += st.Value
	}
	sort.SliceStable(stages, func(i, j int) bool { return stages[i].StageName < stages[j].StageName })
	return contracts.SalesFunnel{
		Stages:         stages,
		OpenValue:      openValue,
		WonCount:       won.Count,
		WonValue:       won.Value,
		LostCount:      lost.Count,
		LostValue:      lost.Value,
		ConversionRate: conversion(won.Count, lost.Count),
	}, nil
}

// Agents returns the seller ranking, ordered by won value desc, with names+avatars.
func (s *SalesMetrics) Agents(ctx context.Context, f contracts.SalesFilter) (contracts.SalesAgents, error) {
	vis, err := s.visibility(ctx)
	if err != nil {
		return contracts.SalesAgents{}, err
	}
	won, err := s.repo.ClosedByAgent(ctx, "won", f, vis)
	if err != nil {
		return contracts.SalesAgents{}, err
	}
	lost, err := s.repo.ClosedByAgent(ctx, "lost", f, vis)
	if err != nil {
		return contracts.SalesAgents{}, err
	}
	open, err := s.repo.OpenByAgent(ctx, f, vis)
	if err != nil {
		return contracts.SalesAgents{}, err
	}

	ids := map[string]struct{}{}
	for id := range won {
		ids[id] = struct{}{}
	}
	for id := range lost {
		ids[id] = struct{}{}
	}
	for id := range open {
		ids[id] = struct{}{}
	}
	rows := make([]contracts.SalesAgent, 0, len(ids))
	for id := range ids {
		if id == "" {
			continue // unassigned deals are not a "seller"
		}
		rows = append(rows, contracts.SalesAgent{
			AgentID:        id,
			WonCount:       won[id].Count,
			WonValue:       won[id].Value,
			LostCount:      lost[id].Count,
			LostValue:      lost[id].Value,
			OpenValue:      open[id].Value,
			ConversionRate: conversion(won[id].Count, lost[id].Count),
		})
	}
	s.labelAgents(ctx, rows)
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].WonValue > rows[j].WonValue })
	return contracts.SalesAgents{Agents: rows}, nil
}

// Cycle returns the cycle-time view: average close time, the per-stage current dwell
// (an approximation — see StageDwell), and the stalled open deals.
func (s *SalesMetrics) Cycle(ctx context.Context, f contracts.SalesFilter, stalledDays int) (contracts.SalesCycle, error) {
	vis, err := s.visibility(ctx)
	if err != nil {
		return contracts.SalesCycle{}, err
	}
	now := s.clock.Now()
	avg, wonCount, err := s.repo.AvgCloseSeconds(ctx, f, vis)
	if err != nil {
		return contracts.SalesCycle{}, err
	}
	dwell, err := s.repo.StageDwell(ctx, now, f, vis)
	if err != nil {
		return contracts.SalesCycle{}, err
	}
	if stalledDays <= 0 {
		stalledDays = defaultStalledDays
	}
	before := now.Add(-time.Duration(stalledDays) * 24 * time.Hour)
	stalledDeals, err := s.repo.StalledOpen(ctx, before, 100, f, vis)
	if err != nil {
		return contracts.SalesCycle{}, err
	}

	s.labelDwell(ctx, f.PipelineID, dwell)
	sort.SliceStable(dwell, func(i, j int) bool { return dwell[i].AvgSeconds > dwell[j].AvgSeconds })

	stalled := make([]contracts.StalledDeal, 0, len(stalledDeals))
	for _, d := range stalledDeals {
		stalled = append(stalled, contracts.StalledDeal{
			ID: d.ID, Title: d.Title, StageID: d.StageID, AssignedTo: d.AssignedTo,
			Value: d.Value, DaysInStage: int(now.Sub(d.StageChangedAt).Hours() / 24),
		})
	}
	s.labelStalled(ctx, f.PipelineID, stalled)
	return contracts.SalesCycle{AvgCloseSeconds: avg, WonCount: wonCount, ByStage: dwell, Stalled: stalled}, nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func (s *SalesMetrics) visibility(ctx context.Context) (contracts.Visibility, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return contracts.Visibility{}, err
	}
	ac, ok := authz.FromContext(ctx)
	if !ok {
		return contracts.Visibility{}, apperror.Unauthorized("authentication required")
	}
	return contracts.Visibility{All: ac.SectorScope == authz.ScopeAll, SectorIDs: ac.SectorIDs, UserID: ac.UserID}, nil
}

// stageNames loads a pipeline's stage id→name map (best-effort; empty on failure or
// when no pipeline is specified — a metrics query may span pipelines).
func (s *SalesMetrics) stageNames(ctx context.Context, pipelineID string) map[string]string {
	out := map[string]string{}
	add := func(p *pipelineentity.Pipeline) {
		for _, st := range p.Stages {
			out[st.ID] = st.Name
		}
	}
	if pipelineID != "" {
		if p, err := s.pipelines.Get(ctx, pipelineID); err == nil {
			add(p)
		}
		return out
	}
	if p, err := s.pipelines.Default(ctx); err == nil {
		add(p)
	}
	return out
}

func (s *SalesMetrics) labelStages(ctx context.Context, pipelineID string, stages []contracts.FunnelStage) {
	names := s.stageNames(ctx, pipelineID)
	for i := range stages {
		stages[i].StageName = names[stages[i].StageID]
	}
}

func (s *SalesMetrics) labelDwell(ctx context.Context, pipelineID string, dwell []contracts.StageDwell) {
	names := s.stageNames(ctx, pipelineID)
	for i := range dwell {
		dwell[i].StageName = names[dwell[i].StageID]
	}
}

func (s *SalesMetrics) labelStalled(ctx context.Context, pipelineID string, stalled []contracts.StalledDeal) {
	names := s.stageNames(ctx, pipelineID)
	for i := range stalled {
		stalled[i].StageName = names[stalled[i].StageID]
	}
	if s.agents == nil {
		return
	}
	ids := make([]string, 0, len(stalled))
	for _, d := range stalled {
		if d.AssignedTo != "" {
			ids = append(ids, d.AssignedTo)
		}
	}
	cards, err := s.agents.AgentCards(ctx, ids)
	if err != nil {
		return
	}
	for i := range stalled {
		if card, ok := cards[stalled[i].AssignedTo]; ok {
			stalled[i].AssignedToName = card.Name
		}
	}
}

func (s *SalesMetrics) labelAgents(ctx context.Context, rows []contracts.SalesAgent) {
	if s.agents == nil {
		return
	}
	ids := make([]string, 0, len(rows))
	for _, a := range rows {
		ids = append(ids, a.AgentID)
	}
	cards, err := s.agents.AgentCards(ctx, ids)
	if err != nil {
		return
	}
	for i := range rows {
		if card, ok := cards[rows[i].AgentID]; ok {
			rows[i].AgentName = card.Name
			rows[i].AgentAvatarURL = card.AvatarURL
		}
	}
}

// conversion returns won / (won + lost), 0 when there are no closed deals.
func conversion(won, lost int) float64 {
	total := won + lost
	if total == 0 {
		return 0
	}
	return float64(won) / float64(total)
}

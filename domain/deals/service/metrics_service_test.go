package service

import (
	"context"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/deals/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/deals/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// fakeAgentDir resolves a fixed set of agent cards, capturing the looked-up ids.
type fakeAgentDir struct {
	cards   map[string]shared.DisplayCard
	gotIDs  []string
	callCnt int
}

func (f *fakeAgentDir) AgentCards(_ context.Context, ids []string) (map[string]shared.DisplayCard, error) {
	f.callCnt++
	f.gotIDs = append(f.gotIDs, ids...)
	return f.cards, nil
}

func newMetrics(repo *fakeRepo) (*SalesMetrics, *fakeAgentDir) {
	dir := &fakeAgentDir{cards: map[string]shared.DisplayCard{
		"u1": {Name: "Ana", AvatarURL: "https://cdn/ana.png"},
		"u2": {Name: "Bruno", AvatarURL: "https://cdn/bruno.png"},
	}}
	svc := NewSalesMetrics(repo, fakePipelines{pl: samplePipeline()}, shared.SystemClock{})
	svc.SetAgentDirectory(dir)
	return svc, dir
}

func TestFunnel_TotalsConversionAndStageNames(t *testing.T) {
	repo := newRepo()
	repo.openByStage = []contracts.FunnelStage{
		{StageID: "s1", Count: 2, Value: 300},
		{StageID: "sw", Count: 1, Value: 100}, // still counted as open snapshot
	}
	repo.closedTotals = map[string]contracts.CountValue{
		"won":  {Count: 3, Value: 900},
		"lost": {Count: 1, Value: 200},
	}
	svc, _ := newMetrics(repo)

	out, err := svc.Funnel(authCtx(authz.ScopeAll, nil, "owner"), contracts.SalesFilter{PipelineID: "p1"})
	if err != nil {
		t.Fatalf("funnel: %v", err)
	}
	if out.WonCount != 3 || out.WonValue != 900 || out.LostCount != 1 || out.LostValue != 200 {
		t.Errorf("won/lost totals wrong: %+v", out)
	}
	if out.OpenValue != 400 {
		t.Errorf("open value must sum the stage values, got %v", out.OpenValue)
	}
	// conversion = won / (won+lost) = 3/4 = 0.75
	if out.ConversionRate != 0.75 {
		t.Errorf("conversion must be won/(won+lost)=0.75, got %v", out.ConversionRate)
	}
	// stage ids resolved to names from the pipeline.
	names := map[string]string{}
	for _, st := range out.Stages {
		names[st.StageID] = st.StageName
	}
	if names["s1"] != "Novo" || names["sw"] != "Ganho" {
		t.Errorf("stage names not resolved in batch: %+v", names)
	}
}

func TestFunnel_NoClosedDealsConversionZero(t *testing.T) {
	repo := newRepo()
	repo.closedTotals = map[string]contracts.CountValue{}
	svc, _ := newMetrics(repo)
	out, err := svc.Funnel(authCtx(authz.ScopeAll, nil, "owner"), contracts.SalesFilter{})
	if err != nil {
		t.Fatalf("funnel: %v", err)
	}
	if out.ConversionRate != 0 {
		t.Errorf("no closed deals must yield 0 conversion, got %v", out.ConversionRate)
	}
}

func TestAgents_RankingEnrichmentAndSort(t *testing.T) {
	repo := newRepo()
	repo.closedByAgent = map[string]map[string]contracts.CountValue{
		"won": {
			"u1": {Count: 2, Value: 500},
			"u2": {Count: 5, Value: 5000},
		},
		"lost": {
			"u1": {Count: 2, Value: 100},
		},
	}
	repo.openByAgent = map[string]contracts.CountValue{
		"u1": {Count: 1, Value: 80},
		"":   {Count: 3, Value: 999}, // unassigned must be skipped
	}
	svc, dir := newMetrics(repo)

	out, err := svc.Agents(authCtx(authz.ScopeAll, nil, "owner"), contracts.SalesFilter{})
	if err != nil {
		t.Fatalf("agents: %v", err)
	}
	if len(out.Agents) != 2 {
		t.Fatalf("unassigned bucket must be dropped, got %d rows: %+v", len(out.Agents), out.Agents)
	}
	// ordered by won value desc → u2 first.
	if out.Agents[0].AgentID != "u2" || out.Agents[1].AgentID != "u1" {
		t.Errorf("must rank by won value desc: %+v", out.Agents)
	}
	if out.Agents[0].AgentName != "Bruno" || out.Agents[0].AgentAvatarURL != "https://cdn/bruno.png" {
		t.Errorf("agent name/avatar not enriched: %+v", out.Agents[0])
	}
	// u1: won 2, lost 2 → conversion 0.5; open value carried.
	u1 := out.Agents[1]
	if u1.ConversionRate != 0.5 || u1.OpenValue != 80 {
		t.Errorf("u1 conversion/open wrong: %+v", u1)
	}
	// one batch directory call.
	if dir.callCnt != 1 {
		t.Errorf("agent directory must be called once (batch), got %d", dir.callCnt)
	}
}

func TestCycle_AvgDwellAndStalled(t *testing.T) {
	repo := newRepo()
	now := time.Now()
	repo.avgClose = 3600
	repo.avgWonCount = 4
	repo.stageDwell = []contracts.StageDwell{
		{StageID: "s1", OpenCount: 3, AvgSeconds: 100},
		{StageID: "sw", OpenCount: 1, AvgSeconds: 900},
	}
	repo.stalled = []*entity.Deal{
		{ID: "d1", Title: "Acme", StageID: "s1", AssignedTo: "u1", Value: 500, StageChangedAt: now.Add(-20 * 24 * time.Hour)},
	}
	svc, _ := newMetrics(repo)

	out, err := svc.Cycle(authCtx(authz.ScopeAll, nil, "owner"), contracts.SalesFilter{PipelineID: "p1"}, 14)
	if err != nil {
		t.Fatalf("cycle: %v", err)
	}
	if out.AvgCloseSeconds != 3600 || out.WonCount != 4 {
		t.Errorf("avg close / won count wrong: %+v", out)
	}
	// dwell sorted by avg seconds desc, names resolved.
	if out.ByStage[0].StageID != "sw" || out.ByStage[0].StageName != "Ganho" {
		t.Errorf("dwell not sorted/labeled: %+v", out.ByStage)
	}
	if len(out.Stalled) != 1 {
		t.Fatalf("expected one stalled deal, got %+v", out.Stalled)
	}
	st := out.Stalled[0]
	if st.StageName != "Novo" || st.AssignedToName != "Ana" {
		t.Errorf("stalled stage/agent names not resolved: %+v", st)
	}
	if st.DaysInStage != 20 {
		t.Errorf("days in stage should be ~20, got %d", st.DaysInStage)
	}
}

func TestSalesMetrics_VisibilityForwarded(t *testing.T) {
	repo := newRepo()
	svc, _ := newMetrics(repo)

	// Restricted scope → narrowed visibility reaches the repo.
	ctx := authCtx(authz.ScopeOwn, []string{"sec1"}, "u9")
	if _, err := svc.Funnel(ctx, contracts.SalesFilter{}); err != nil {
		t.Fatalf("funnel: %v", err)
	}
	if repo.gotSalesVis.All || repo.gotSalesVis.UserID != "u9" || len(repo.gotSalesVis.SectorIDs) != 1 {
		t.Errorf("own-scope visibility not forwarded: %+v", repo.gotSalesVis)
	}

	// All scope → All=true.
	if _, err := svc.Agents(authCtx(authz.ScopeAll, nil, "owner"), contracts.SalesFilter{}); err != nil {
		t.Fatalf("agents: %v", err)
	}
	if !repo.gotSalesVis.All {
		t.Errorf("all-scope visibility must be All=true: %+v", repo.gotSalesVis)
	}
}

func TestSalesMetrics_RequiresTenant(t *testing.T) {
	repo := newRepo()
	svc, _ := newMetrics(repo)
	if _, err := svc.Funnel(context.Background(), contracts.SalesFilter{}); err == nil {
		t.Error("missing tenant/auth must error")
	}
}

package service

import (
	"context"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/reports/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/reports/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

// fakeRepo returns canned aggregates and records the filter it received.
type fakeRepo struct {
	gotFilter contracts.Filter
	total     int
	open      []contracts.Bucket
	msgs      int
	frAvg     float64
	resAvg    float64
	csat      repository.CSATRaw
	sla       contracts.SLAReport
	byStatus  []contracts.Bucket
	bySector  []contracts.Bucket
	daily     []contracts.DateCount
	byChannel []contracts.Bucket
	closedBy  []contracts.Bucket
	agents    []contracts.AgentStat
	sectors   []contracts.SectorStat
	autom     []contracts.Bucket
	copilot   contracts.CopilotReport
}

func (r *fakeRepo) CountConversations(_ context.Context, f contracts.Filter) (int, error) {
	r.gotFilter = f
	return r.total, nil
}
func (r *fakeRepo) CountMessages(context.Context, contracts.Filter) (int, error) { return r.msgs, nil }
func (r *fakeRepo) OpenByStatus(context.Context, contracts.Filter) ([]contracts.Bucket, error) {
	return r.open, nil
}
func (r *fakeRepo) ConversationsByStatus(context.Context, contracts.Filter) ([]contracts.Bucket, error) {
	return r.byStatus, nil
}
func (r *fakeRepo) ConversationsDaily(context.Context, contracts.Filter) ([]contracts.DateCount, error) {
	return r.daily, nil
}
func (r *fakeRepo) ConversationsBySector(context.Context, contracts.Filter) ([]contracts.Bucket, error) {
	return r.bySector, nil
}
func (r *fakeRepo) ClosedByReason(context.Context, contracts.Filter) ([]contracts.Bucket, error) {
	return r.closedBy, nil
}
func (r *fakeRepo) MessagesByChannel(context.Context, contracts.Filter) ([]contracts.Bucket, error) {
	return r.byChannel, nil
}
func (r *fakeRepo) FirstResponseAvgSeconds(context.Context, contracts.Filter) (float64, error) {
	return r.frAvg, nil
}
func (r *fakeRepo) ResolutionAvgSeconds(context.Context, contracts.Filter) (float64, error) {
	return r.resAvg, nil
}
func (r *fakeRepo) AgentStats(context.Context, contracts.Filter) ([]contracts.AgentStat, error) {
	return r.agents, nil
}
func (r *fakeRepo) SectorStats(context.Context, contracts.Filter) ([]contracts.SectorStat, error) {
	return r.sectors, nil
}
func (r *fakeRepo) AutomationByStatus(context.Context, contracts.Filter) ([]contracts.Bucket, error) {
	return r.autom, nil
}
func (r *fakeRepo) CopilotUsage(context.Context, contracts.Filter) (contracts.CopilotReport, error) {
	return r.copilot, nil
}
func (r *fakeRepo) SLACounts(context.Context, contracts.Filter) (contracts.SLAReport, error) {
	return r.sla, nil
}
func (r *fakeRepo) CSAT(context.Context, contracts.Filter) (repository.CSATRaw, error) {
	return r.csat, nil
}

func ctxT() context.Context { return shared.WithTenant(context.Background(), "t1") }

const now = "2026-06-11T12:00:00Z"

func clk(t *testing.T) fixedClock {
	tm, _ := time.Parse(time.RFC3339, now)
	return fixedClock{t: tm}
}

func TestOverview_ComposesAndDerivesRates(t *testing.T) {
	repo := &fakeRepo{
		total:  100,
		open:   []contracts.Bucket{{Key: "assigned", Count: 7}},
		msgs:   500,
		frAvg:  42.5,
		resAvg: 3600,
		csat:   repository.CSATRaw{Sent: 20, Responded: 15, ScoreSum: 60, ScoreCount: 15},
		sla:    contracts.SLAReport{Tracked: 50, FirstResponseBreached: 5, ResolutionBreached: 10},
	}
	svc := NewService(repo, clk(t))
	ov, err := svc.Overview(ctxT(), contracts.Filter{})
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if ov.TotalConversations != 100 || ov.Messages != 500 {
		t.Errorf("unexpected totals: %+v", ov)
	}
	if ov.FirstResponseAvgSec != 42.5 || ov.ResolutionAvgSec != 3600 {
		t.Errorf("unexpected averages: %+v", ov)
	}
	if ov.CSATAvgScore != 4 { // 60/15
		t.Errorf("csat avg = %v, want 4", ov.CSATAvgScore)
	}
	if ov.CSATResponseRate != 0.75 { // 15/20
		t.Errorf("csat response rate = %v, want 0.75", ov.CSATResponseRate)
	}
	if ov.SLAFirstResponseBreachRate != 0.1 || ov.SLAResolutionBreachRate != 0.2 {
		t.Errorf("sla rates = %v/%v, want 0.1/0.2", ov.SLAFirstResponseBreachRate, ov.SLAResolutionBreachRate)
	}
}

func TestNormalize_DefaultsLast30Days(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo, clk(t))
	if _, err := svc.Overview(ctxT(), contracts.Filter{}); err != nil {
		t.Fatalf("overview: %v", err)
	}
	wantTo, _ := time.Parse(time.RFC3339, now)
	if !repo.gotFilter.To.Equal(wantTo) {
		t.Errorf("To = %s, want now", repo.gotFilter.To)
	}
	if !repo.gotFilter.From.Equal(wantTo.Add(-30 * 24 * time.Hour)) {
		t.Errorf("From = %s, want now-30d", repo.gotFilter.From)
	}
}

func TestNormalize_KeepsExplicitPeriodAndFilters(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo, clk(t))
	from, _ := time.Parse(time.RFC3339, "2026-01-01T00:00:00Z")
	to, _ := time.Parse(time.RFC3339, "2026-02-01T00:00:00Z")
	f := contracts.Filter{From: from, To: to, SectorID: "s1", Channel: "whatsapp"}
	if _, err := svc.Overview(ctxT(), f); err != nil {
		t.Fatalf("overview: %v", err)
	}
	if !repo.gotFilter.From.Equal(from) || !repo.gotFilter.To.Equal(to) {
		t.Errorf("explicit period not preserved: %+v", repo.gotFilter)
	}
	if repo.gotFilter.SectorID != "s1" || repo.gotFilter.Channel != "whatsapp" {
		t.Errorf("filters not passed through: %+v", repo.gotFilter)
	}
}

func TestSLA_DerivesBreachRates(t *testing.T) {
	repo := &fakeRepo{sla: contracts.SLAReport{Tracked: 8, FirstResponseBreached: 2, ResolutionBreached: 1, Met: 5}}
	svc := NewService(repo, clk(t))
	r, err := svc.SLA(ctxT(), contracts.Filter{})
	if err != nil {
		t.Fatalf("sla: %v", err)
	}
	if r.FirstResponseBreachRate != 0.25 || r.ResolutionBreachRate != 0.125 {
		t.Errorf("breach rates = %v/%v, want 0.25/0.125", r.FirstResponseBreachRate, r.ResolutionBreachRate)
	}
}

func TestCSAT_DerivesAvgAndRate(t *testing.T) {
	repo := &fakeRepo{csat: repository.CSATRaw{
		Sent: 10, Responded: 4, Expired: 6, ScoreSum: 14, ScoreCount: 4,
		ByScore: []contracts.Bucket{{Key: "5", Count: 2}, {Key: "2", Count: 2}},
	}}
	svc := NewService(repo, clk(t))
	r, err := svc.CSAT(ctxT(), contracts.Filter{})
	if err != nil {
		t.Fatalf("csat: %v", err)
	}
	if r.AvgScore != 3.5 || r.ResponseRate != 0.4 {
		t.Errorf("csat avg/rate = %v/%v, want 3.5/0.4", r.AvgScore, r.ResponseRate)
	}
	if len(r.ByScore) != 2 {
		t.Errorf("score distribution lost")
	}
}

func TestAutomation_SumsTotal(t *testing.T) {
	repo := &fakeRepo{autom: []contracts.Bucket{{Key: "completed", Count: 7}, {Key: "failed", Count: 3}}}
	svc := NewService(repo, clk(t))
	r, err := svc.Automation(ctxT(), contracts.Filter{})
	if err != nil {
		t.Fatalf("automation: %v", err)
	}
	if r.Total != 10 {
		t.Errorf("total = %d, want 10", r.Total)
	}
}

func TestZeroDenominators_NoDivideByZero(t *testing.T) {
	svc := NewService(&fakeRepo{}, clk(t))
	ov, _ := svc.Overview(ctxT(), contracts.Filter{})
	if ov.CSATAvgScore != 0 || ov.CSATResponseRate != 0 || ov.SLAFirstResponseBreachRate != 0 {
		t.Errorf("empty data must yield zero rates, got %+v", ov)
	}
}

func TestReports_RequireTenant(t *testing.T) {
	svc := NewService(&fakeRepo{}, clk(t))
	if _, err := svc.Overview(context.Background(), contracts.Filter{}); apperror.From(err).Code != apperror.CodeForbidden {
		t.Errorf("expected forbidden without tenant, got %v", err)
	}
}

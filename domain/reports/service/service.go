// Package service implements the ReportService over the aggregation Repository.
// It normalizes the period, runs the aggregations and computes derived metrics
// (averages, rates) so those stay testable without a database.
package service

import (
	"context"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/reports/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/reports/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// defaultPeriod is used when the caller does not bound the period.
const defaultPeriod = 30 * 24 * time.Hour

// Service is the Mongo-backed ReportService.
type Service struct {
	repo    repository.Repository
	clock   shared.Clock
	auditor shared.Auditor
	export  contracts.ExportEnqueuer
}

// NewService builds the service.
func NewService(repo repository.Repository, clock shared.Clock) *Service {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &Service{repo: repo, clock: clock, auditor: shared.NoopAuditor{}}
}

// SetAuditor wires the audit trail. Optional: when unset, report exports are not
// audited.
func (s *Service) SetAuditor(a shared.Auditor) {
	if a != nil {
		s.auditor = a
	}
}

// SetExportEnqueuer wires the async report-export enqueuer. Optional: when unset,
// RequestExport audits the request but does not enqueue a job.
func (s *Service) SetExportEnqueuer(e contracts.ExportEnqueuer) {
	if e != nil {
		s.export = e
	}
}

// RequestExport audits a report-export request ("exportação de relatório") and
// enqueues the (prepared) reports.export job. The file generation is future work.
func (s *Service) RequestExport(ctx context.Context, report, format string, f contracts.Filter) error {
	if err := s.guard(ctx); err != nil {
		return err
	}
	f = s.normalize(f)
	actor := ""
	if ac, ok := authz.FromContext(ctx); ok {
		actor = ac.UserID
	}
	if err := s.auditor.Record(ctx, shared.AuditEntry{
		Action: "report.export", ResourceType: "report", ResourceID: report,
		Data: map[string]any{"format": format, "sector_id": f.SectorID, "channel": f.Channel},
	}); err != nil {
		return err
	}
	if s.export != nil {
		tenantID, _ := shared.TenantFrom(ctx)
		return s.export.EnqueueExport(contracts.ExportTask{
			TenantID: tenantID, ActorID: actor, Report: report, Format: format,
			From: f.From.UnixMilli(), To: f.To.UnixMilli(), SectorID: f.SectorID, Channel: f.Channel,
		})
	}
	return nil
}

// normalize fills a missing period with the last 30 days (To defaults to now).
func (s *Service) normalize(f contracts.Filter) contracts.Filter {
	if f.To.IsZero() {
		f.To = s.clock.Now()
	}
	if f.From.IsZero() {
		f.From = f.To.Add(-defaultPeriod)
	}
	return f
}

func (s *Service) guard(ctx context.Context) error {
	_, err := shared.RequireTenant(ctx)
	return err
}

// Overview composes the headline summary.
func (s *Service) Overview(ctx context.Context, f contracts.Filter) (contracts.Overview, error) {
	if err := s.guard(ctx); err != nil {
		return contracts.Overview{}, err
	}
	f = s.normalize(f)

	total, err := s.repo.CountConversations(ctx, f)
	if err != nil {
		return contracts.Overview{}, err
	}
	open, err := s.repo.OpenByStatus(ctx, f)
	if err != nil {
		return contracts.Overview{}, err
	}
	msgs, err := s.repo.CountMessages(ctx, f)
	if err != nil {
		return contracts.Overview{}, err
	}
	frAvg, err := s.repo.FirstResponseAvgSeconds(ctx, f)
	if err != nil {
		return contracts.Overview{}, err
	}
	resAvg, err := s.repo.ResolutionAvgSeconds(ctx, f)
	if err != nil {
		return contracts.Overview{}, err
	}
	csat, err := s.repo.CSAT(ctx, f)
	if err != nil {
		return contracts.Overview{}, err
	}
	sla, err := s.repo.SLACounts(ctx, f)
	if err != nil {
		return contracts.Overview{}, err
	}

	return contracts.Overview{
		From: f.From, To: f.To,
		TotalConversations:         total,
		OpenByStatus:               open,
		Messages:                   msgs,
		FirstResponseAvgSec:        frAvg,
		ResolutionAvgSec:           resAvg,
		CSATAvgScore:               avg(csat.ScoreSum, csat.ScoreCount),
		CSATResponseRate:           rate(csat.Responded, csat.Sent),
		SLAFirstResponseBreachRate: rate(sla.FirstResponseBreached, sla.Tracked),
		SLAResolutionBreachRate:    rate(sla.ResolutionBreached, sla.Tracked),
	}, nil
}

// Conversations breaks conversations down several ways.
func (s *Service) Conversations(ctx context.Context, f contracts.Filter) (contracts.ConversationsReport, error) {
	if err := s.guard(ctx); err != nil {
		return contracts.ConversationsReport{}, err
	}
	f = s.normalize(f)
	out := contracts.ConversationsReport{}
	var err error
	if out.Daily, err = s.repo.ConversationsDaily(ctx, f); err != nil {
		return out, err
	}
	if out.ByStatus, err = s.repo.ConversationsByStatus(ctx, f); err != nil {
		return out, err
	}
	if out.BySector, err = s.repo.ConversationsBySector(ctx, f); err != nil {
		return out, err
	}
	if out.ByChannel, err = s.repo.MessagesByChannel(ctx, f); err != nil {
		return out, err
	}
	if out.ClosedByReason, err = s.repo.ClosedByReason(ctx, f); err != nil {
		return out, err
	}
	return out, nil
}

// Agents returns the per-agent breakdown.
func (s *Service) Agents(ctx context.Context, f contracts.Filter) (contracts.AgentsReport, error) {
	if err := s.guard(ctx); err != nil {
		return contracts.AgentsReport{}, err
	}
	f = s.normalize(f)
	agents, err := s.repo.AgentStats(ctx, f)
	if err != nil {
		return contracts.AgentsReport{}, err
	}
	return contracts.AgentsReport{Agents: agents}, nil
}

// Sectors returns the per-sector breakdown.
func (s *Service) Sectors(ctx context.Context, f contracts.Filter) (contracts.SectorsReport, error) {
	if err := s.guard(ctx); err != nil {
		return contracts.SectorsReport{}, err
	}
	f = s.normalize(f)
	sectors, err := s.repo.SectorStats(ctx, f)
	if err != nil {
		return contracts.SectorsReport{}, err
	}
	return contracts.SectorsReport{Sectors: sectors}, nil
}

// Automation summarizes automation runs.
func (s *Service) Automation(ctx context.Context, f contracts.Filter) (contracts.AutomationReport, error) {
	if err := s.guard(ctx); err != nil {
		return contracts.AutomationReport{}, err
	}
	f = s.normalize(f)
	by, err := s.repo.AutomationByStatus(ctx, f)
	if err != nil {
		return contracts.AutomationReport{}, err
	}
	total := 0
	for _, b := range by {
		total += b.Count
	}
	return contracts.AutomationReport{Total: total, ByStatus: by}, nil
}

// Copilot summarizes AI copilot usage.
func (s *Service) Copilot(ctx context.Context, f contracts.Filter) (contracts.CopilotReport, error) {
	if err := s.guard(ctx); err != nil {
		return contracts.CopilotReport{}, err
	}
	f = s.normalize(f)
	return s.repo.CopilotUsage(ctx, f)
}

// SLA summarizes SLA outcomes with derived breach rates.
func (s *Service) SLA(ctx context.Context, f contracts.Filter) (contracts.SLAReport, error) {
	if err := s.guard(ctx); err != nil {
		return contracts.SLAReport{}, err
	}
	f = s.normalize(f)
	r, err := s.repo.SLACounts(ctx, f)
	if err != nil {
		return contracts.SLAReport{}, err
	}
	r.FirstResponseBreachRate = rate(r.FirstResponseBreached, r.Tracked)
	r.ResolutionBreachRate = rate(r.ResolutionBreached, r.Tracked)
	return r, nil
}

// CSAT summarizes satisfaction surveys with derived average and response rate.
func (s *Service) CSAT(ctx context.Context, f contracts.Filter) (contracts.CSATReport, error) {
	if err := s.guard(ctx); err != nil {
		return contracts.CSATReport{}, err
	}
	f = s.normalize(f)
	raw, err := s.repo.CSAT(ctx, f)
	if err != nil {
		return contracts.CSATReport{}, err
	}
	return contracts.CSATReport{
		Sent:         raw.Sent,
		Responded:    raw.Responded,
		Expired:      raw.Expired,
		AvgScore:     avg(raw.ScoreSum, raw.ScoreCount),
		ResponseRate: rate(raw.Responded, raw.Sent),
		ByScore:      raw.ByScore,
	}, nil
}

// avg returns sum/count rounded to two decimals, or 0 when count is 0.
func avg(sum, count int) float64 {
	if count == 0 {
		return 0
	}
	return round2(float64(sum) / float64(count))
}

// rate returns num/den rounded to four decimals, or 0 when den is 0.
func rate(num, den int) float64 {
	if den == 0 {
		return 0
	}
	return round4(float64(num) / float64(den))
}

func round2(v float64) float64 { return float64(int64(v*100+0.5)) / 100 }
func round4(v float64) float64 { return float64(int64(v*10000+0.5)) / 10000 }

var _ contracts.ReportService = (*Service)(nil)

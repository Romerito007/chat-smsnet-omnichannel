// Package service implements the ReportService over the aggregation Repository.
// It normalizes the period, runs the aggregations and computes derived metrics
// (averages, rates) so those stay testable without a database.
package service

import (
	"context"
	"strings"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/reports/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/reports/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// defaultPeriod is used when the caller does not bound the period.
const defaultPeriod = 30 * 24 * time.Hour

// defaultExportTTL bounds how long an export's signed URL stays valid.
const defaultExportTTL = 24 * time.Hour

// Service is the Mongo-backed ReportService.
type Service struct {
	repo    repository.Repository
	clock   shared.Clock
	auditor shared.Auditor
	files   contracts.FileStore
	urlTTL  time.Duration
	agents  AgentDirectory
	sectors NameDirectory
	reasons NameDirectory
}

// AgentDirectory resolves agent ids to display cards (name + signed avatar URL) so
// the per-agent rows render the agent instead of a raw id. Satisfied by the IAM
// user service (AgentCards). Optional.
type AgentDirectory interface {
	AgentCards(ctx context.Context, userIDs []string) (map[string]shared.DisplayCard, error)
}

// NameDirectory resolves ids to display names (sector, close reason). Satisfied by
// the sector and close-reason services (Names). Optional.
type NameDirectory interface {
	Names(ctx context.Context, ids []string) (map[string]string, error)
}

// SetDirectories wires the agent, sector and close-reason directories used to
// resolve raw ids to display names/labels on every report — for BOTH the GET reads
// and the file export, so they stay consistent. Optional: when unset, rows/buckets
// carry only the raw id.
func (s *Service) SetDirectories(agents AgentDirectory, sectors, reasons NameDirectory) *Service {
	s.agents = agents
	s.sectors = sectors
	s.reasons = reasons
	return s
}

// NewService builds the service.
func NewService(repo repository.Repository, clock shared.Clock) *Service {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &Service{repo: repo, clock: clock, auditor: shared.NoopAuditor{}, urlTTL: defaultExportTTL}
}

// SetAuditor wires the audit trail. Optional: when unset, report exports are not
// audited.
func (s *Service) SetAuditor(a shared.Auditor) {
	if a != nil {
		s.auditor = a
	}
}

// SetFileStore wires the export file store and the signed-URL lifetime. Export
// requires it; without it Export returns an error rather than an empty result.
func (s *Service) SetFileStore(files contracts.FileStore, ttl time.Duration) {
	s.files = files
	if ttl > 0 {
		s.urlTTL = ttl
	}
}

// Export renders the named report into a real file (json|csv), writes it to the
// file store and returns a temporary signed download URL. The request is audited.
func (s *Service) Export(ctx context.Context, report, format string, f contracts.Filter) (contracts.ExportResult, error) {
	if err := s.guard(ctx); err != nil {
		return contracts.ExportResult{}, err
	}
	if s.files == nil {
		return contracts.ExportResult{}, apperror.Internal("report export storage is not configured")
	}
	tenantID, _ := shared.TenantFrom(ctx)
	f = s.normalize(f)

	data, ext, err := s.render(ctx, report, format, f)
	if err != nil {
		return contracts.ExportResult{}, err
	}

	filename := report + "-" + s.clock.Now().UTC().Format("20060102-150405") + "." + ext
	key := strings.Join([]string{"reports", tenantID, shared.NewID() + "-" + filename}, "/")
	if err := s.files.Save(key, data); err != nil {
		return contracts.ExportResult{}, apperror.Internal("could not write report file").Wrap(err)
	}
	url, expiresAt, err := s.files.SignedURL(key, s.urlTTL)
	if err != nil {
		return contracts.ExportResult{}, apperror.Internal("could not sign report url").Wrap(err)
	}

	if err := s.auditor.Record(ctx, shared.AuditEntry{
		Action: "report.export", ResourceType: "report", ResourceID: report,
		Data: map[string]any{"format": ext, "bytes": len(data), "sector_id": f.SectorID, "channel": f.Channel},
	}); err != nil {
		return contracts.ExportResult{}, err
	}

	return contracts.ExportResult{
		Report: report, Format: ext, Filename: filename,
		DownloadURL: url, ExpiresAt: expiresAt, Bytes: len(data),
	}, nil
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
	// Label the id-keyed buckets (by_sector, closed_by_reason) so reads and exports
	// render names; by_status/messages_by_channel keys are already human-readable.
	s.labelBuckets(ctx, out.BySector, s.sectors)
	s.labelBuckets(ctx, out.ClosedByReason, s.reasons)
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
	rep := contracts.AgentsReport{Agents: agents}
	s.enrichAgents(ctx, rep.Agents)
	return rep, nil
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
	rep := contracts.SectorsReport{Sectors: sectors}
	s.enrichSectors(ctx, rep.Sectors)
	return rep, nil
}

// enrichAgents fills each agent row's display name + avatar from the agent
// directory, in ONE batch call. Best-effort: a nil directory or a lookup error
// leaves the rows with their raw ids.
func (s *Service) enrichAgents(ctx context.Context, agents []contracts.AgentStat) {
	if s.agents == nil || len(agents) == 0 {
		return
	}
	ids := make([]string, 0, len(agents))
	for _, a := range agents {
		if a.AgentID != "" {
			ids = append(ids, a.AgentID)
		}
	}
	cards, err := s.agents.AgentCards(ctx, ids)
	if err != nil {
		return
	}
	for i := range agents {
		if card, ok := cards[agents[i].AgentID]; ok {
			agents[i].Name = card.Name
			agents[i].AvatarURL = card.AvatarURL
		}
	}
}

// enrichSectors fills each sector row's display name, in ONE batch call. Best-effort.
func (s *Service) enrichSectors(ctx context.Context, sectors []contracts.SectorStat) {
	if s.sectors == nil || len(sectors) == 0 {
		return
	}
	ids := make([]string, 0, len(sectors))
	for _, sec := range sectors {
		if sec.SectorID != "" {
			ids = append(ids, sec.SectorID)
		}
	}
	names, err := s.sectors.Names(ctx, ids)
	if err != nil {
		return
	}
	for i := range sectors {
		if name, ok := names[sectors[i].SectorID]; ok {
			sectors[i].Name = name
		}
	}
}

// labelBuckets fills each bucket's Label from the resolved name of its key (an id),
// in ONE batch call. Best-effort: a nil directory or a lookup error leaves the
// buckets with only their raw key.
func (s *Service) labelBuckets(ctx context.Context, buckets []contracts.Bucket, dir NameDirectory) {
	if dir == nil || len(buckets) == 0 {
		return
	}
	ids := make([]string, 0, len(buckets))
	for _, b := range buckets {
		if b.Key != "" {
			ids = append(ids, b.Key)
		}
	}
	names, err := dir.Names(ctx, ids)
	if err != nil {
		return
	}
	for i := range buckets {
		if name, ok := names[buckets[i].Key]; ok {
			buckets[i].Label = name
		}
	}
}

// Copilot summarizes AI copilot usage.
func (s *Service) Copilot(ctx context.Context, f contracts.Filter) (contracts.CopilotReport, error) {
	if err := s.guard(ctx); err != nil {
		return contracts.CopilotReport{}, err
	}
	f = s.normalize(f)
	return s.repo.CopilotUsage(ctx, f)
}

// Automation summarizes automation-rule firings over the period.
func (s *Service) Automation(ctx context.Context, f contracts.Filter) (contracts.AutomationReport, error) {
	if err := s.guard(ctx); err != nil {
		return contracts.AutomationReport{}, err
	}
	f = s.normalize(f)
	return s.repo.AutomationUsage(ctx, f)
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

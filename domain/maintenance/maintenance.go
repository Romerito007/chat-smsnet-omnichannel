// Package maintenance holds the periodic housekeeping jobs that have no natural
// home in a feature domain yet: the reports pre-aggregation snapshot
// (reports.snapshot — superseded by the reports domain in a later prompt) and
// audit-log retention (audit.compact). Logic lives here; the scheduler only
// schedules and the worker dispatches.
package maintenance

import (
	"context"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// Snapshot is a per-tenant, per-day pre-aggregation of headline metrics.
type Snapshot struct {
	TenantID            string
	Date                string // YYYY-MM-DD (UTC)
	OpenConversations   int
	ClosedConversations int
	Messages            int
	GeneratedAt         time.Time
}

// Counts is the raw aggregation for a day.
type Counts struct {
	OpenConversations   int
	ClosedConversations int
	Messages            int
}

// Repository is the maintenance persistence port.
type Repository interface {
	// DeleteAuditBefore removes audit_logs created at or before the cutoff for the
	// tenant. Returns the count deleted.
	DeleteAuditBefore(ctx context.Context, before time.Time) (int, error)
	// DayCounts aggregates the tenant's metrics for the [from, to) window.
	DayCounts(ctx context.Context, from, to time.Time) (Counts, error)
	// UpsertSnapshot writes (idempotently) the per-tenant/day snapshot.
	UpsertSnapshot(ctx context.Context, s Snapshot) error
}

// Service runs the maintenance jobs for a tenant (the worker fans out across
// tenants).
type Service struct {
	repo  Repository
	clock shared.Clock
}

// NewService builds the service.
func NewService(repo Repository, clock shared.Clock) *Service {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &Service{repo: repo, clock: clock}
}

// CompactAudit deletes audit logs older than retention. Idempotent. Returns the
// count removed.
func (s *Service) CompactAudit(ctx context.Context, retention time.Duration) (int, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return 0, err
	}
	return s.repo.DeleteAuditBefore(ctx, s.clock.Now().Add(-retention))
}

// SnapshotDay pre-aggregates the metrics for the UTC day containing `at` and
// upserts the snapshot. Idempotent (one snapshot per tenant/day).
func (s *Service) SnapshotDay(ctx context.Context, at time.Time) (Snapshot, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return Snapshot{}, err
	}
	tenantID, _ := shared.TenantFrom(ctx)
	from := dayStartUTC(at)
	to := from.AddDate(0, 0, 1)
	counts, err := s.repo.DayCounts(ctx, from, to)
	if err != nil {
		return Snapshot{}, err
	}
	snap := Snapshot{
		TenantID:            tenantID,
		Date:                from.Format("2006-01-02"),
		OpenConversations:   counts.OpenConversations,
		ClosedConversations: counts.ClosedConversations,
		Messages:            counts.Messages,
		GeneratedAt:         s.clock.Now(),
	}
	if err := s.repo.UpsertSnapshot(ctx, snap); err != nil {
		return Snapshot{}, err
	}
	return snap, nil
}

func dayStartUTC(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

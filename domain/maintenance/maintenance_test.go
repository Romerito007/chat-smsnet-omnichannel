package maintenance

import (
	"context"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type fakeRepo struct {
	deletedBefore time.Time
	deleteCount   int
	counts        Counts
	from, to      time.Time
	upserted      *Snapshot
}

func (r *fakeRepo) DeleteAuditBefore(_ context.Context, before time.Time) (int, error) {
	r.deletedBefore = before
	return r.deleteCount, nil
}
func (r *fakeRepo) DayCounts(_ context.Context, from, to time.Time) (Counts, error) {
	r.from, r.to = from, to
	return r.counts, nil
}
func (r *fakeRepo) UpsertSnapshot(_ context.Context, s Snapshot) error {
	cp := s
	r.upserted = &cp
	return nil
}

func ctxT() context.Context { return shared.WithTenant(context.Background(), "t1") }

func TestCompactAudit_DeletesBeforeRetentionCutoff(t *testing.T) {
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	repo := &fakeRepo{deleteCount: 7}
	svc := NewService(repo, fixedClock{t: now})

	n, err := svc.CompactAudit(ctxT(), 24*time.Hour)
	if err != nil {
		t.Fatalf("compact: %v", err)
	}
	if n != 7 {
		t.Errorf("expected 7 deleted, got %d", n)
	}
	if !repo.deletedBefore.Equal(now.Add(-24 * time.Hour)) {
		t.Errorf("cutoff = %s, want now-24h", repo.deletedBefore)
	}
}

func TestSnapshotDay_AggregatesUTCDayAndUpserts(t *testing.T) {
	// 13:00 UTC on 2026-06-11 → the snapshot day is [2026-06-11T00:00Z, +1d).
	now := time.Date(2026, 6, 11, 13, 0, 0, 0, time.UTC)
	repo := &fakeRepo{counts: Counts{OpenConversations: 5, ClosedConversations: 3, Messages: 42}}
	svc := NewService(repo, fixedClock{t: now})

	snap, err := svc.SnapshotDay(ctxT(), now)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	wantFrom := time.Date(2026, 6, 11, 0, 0, 0, 0, time.UTC)
	if !repo.from.Equal(wantFrom) || !repo.to.Equal(wantFrom.AddDate(0, 0, 1)) {
		t.Errorf("aggregation window = [%s,%s), want full UTC day", repo.from, repo.to)
	}
	if snap.Date != "2026-06-11" || snap.OpenConversations != 5 || snap.ClosedConversations != 3 || snap.Messages != 42 {
		t.Errorf("unexpected snapshot: %+v", snap)
	}
	if repo.upserted == nil || repo.upserted.TenantID != "t1" {
		t.Errorf("snapshot not upserted for the tenant: %+v", repo.upserted)
	}
}
